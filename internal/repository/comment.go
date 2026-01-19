package repository

import (
	"database/sql"
	"errors"

	"github.com/freshtea599/PhotoHubServer.git/internal/models"
)

type CommentRepository struct {
	db *sql.DB
}

func NewCommentRepository(db *sql.DB) *CommentRepository {
	return &CommentRepository{db: db}
}

// создаёт комментарий и возвращает его с username
func (r *CommentRepository) CreateComment(photoID, userID int64, text string) (*models.Comment, error) {
	var comment models.Comment
	var username string
	// Сначала вставляем комментарий
	insertErr := r.db.QueryRow(`
		INSERT INTO comments (photo_id, user_id, text, likes_count, created_at, updated_at)
		VALUES ($1, $2, $3, 0, NOW(), NOW())
		RETURNING id, photo_id, user_id, text, likes_count, created_at, updated_at
	`, photoID, userID, text).Scan(
		&comment.ID,
		&comment.PhotoID,
		&comment.UserID,
		&comment.Text,
		&comment.LikesCount,
		&comment.CreatedAt,
		&comment.UpdatedAt,
	)

	if insertErr != nil {
		return nil, insertErr
	}

	// Потом получаем username пользователя
	getErr := r.db.QueryRow(`
		SELECT username FROM users WHERE id = $1
	`, userID).Scan(&username)

	if getErr != nil {
		return nil, getErr
	}

	comment.Username = username
	comment.UserLiked = false
	return &comment, nil
}

// получает комментарии фото с информацией о пользователе
func (r *CommentRepository) GetCommentsByPhoto(photoID, currentUserID int64) ([]*models.Comment, error) {
	rows, err := r.db.Query(`
		SELECT 
			c.id,
			c.photo_id,
			c.user_id,
			c.text,
			c.likes_count,
			c.created_at,
			c.updated_at,
			u.username,
			CASE 
				WHEN cl.id IS NOT NULL THEN true 
				ELSE false 
			END as user_liked
		FROM comments c
		JOIN users u ON c.user_id = u.id
		LEFT JOIN comment_likes cl ON cl.comment_id = c.id AND cl.user_id = $2
		WHERE c.photo_id = $1
		ORDER BY c.created_at DESC
	`, photoID, currentUserID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []*models.Comment
	for rows.Next() {
		var comment models.Comment
		var username string
		var userLiked bool

		err := rows.Scan(
			&comment.ID,
			&comment.PhotoID,
			&comment.UserID,
			&comment.Text,
			&comment.LikesCount,
			&comment.CreatedAt,
			&comment.UpdatedAt,
			&username,
			&userLiked,
		)
		if err != nil {
			return nil, err
		}

		comment.Username = username
		comment.UserLiked = userLiked

		comments = append(comments, &comment)
	}

	return comments, rows.Err()
}

// ставит лайк комментарию
func (r *CommentRepository) LikeComment(commentID, userID int64) error {
	_, err := r.db.Exec(
		`INSERT INTO comment_likes (comment_id, user_id, created_at) VALUES ($1, $2, NOW())
		 ON CONFLICT DO NOTHING`,
		commentID, userID,
	)
	if err != nil {
		return err
	}

	// Обновляем счётчик
	_, err = r.db.Exec(
		`UPDATE comments SET likes_count = (SELECT COUNT(*) FROM comment_likes WHERE comment_id = $1) WHERE id = $1`,
		commentID,
	)
	return err
}

// удаляет лайк комментария
func (r *CommentRepository) UnlikeComment(commentID, userID int64) error {
	_, err := r.db.Exec(
		`DELETE FROM comment_likes WHERE comment_id = $1 AND user_id = $2`,
		commentID, userID,
	)
	if err != nil {
		return err
	}

	// Обновляем счётчик
	_, err = r.db.Exec(
		`UPDATE comments SET likes_count = (SELECT COUNT(*) FROM comment_likes WHERE comment_id = $1) WHERE id = $1`,
		commentID,
	)
	return err
}

// проверяет, лайкнул ли юзер комментарий
func (r *CommentRepository) IsCommentLikedByUser(commentID, userID int64) (bool, error) {
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM comment_likes WHERE comment_id = $1 AND user_id = $2`,
		commentID, userID,
	).Scan(&count)
	return count > 0, err
}

// удаляет комментарий
func (r *CommentRepository) DeleteComment(commentID int64) error {
	result, err := r.db.Exec(`DELETE FROM comments WHERE id = $1`, commentID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("comment not found")
	}

	return nil
}

// жалуется на комментарий
func (r *CommentRepository) ReportComment(commentID, reportedBy int64, reason string) error {
	_, err := r.db.Exec(
		`INSERT INTO comment_reports (comment_id, reported_by, reason, status, created_at, updated_at)
		 VALUES ($1, $2, $3, 'pending', NOW(), NOW())`,
		commentID, reportedBy, reason,
	)
	return err
}

// получает жалобы (для админа)
func (r *CommentRepository) GetCommentReports(status string) ([]*models.CommentReport, error) {
	rows, err := r.db.Query(
		`SELECT cr.id,
		        cr.comment_id,
		        cr.reported_by,
		        cr.reason,
		        cr.status,
		        COALESCE(cr.admin_note, ''),
		        cr.created_at,
		        c.text,
		        c.user_id,
		        u.username
		 FROM comment_reports cr
		 JOIN comments c ON c.id = cr.comment_id
		 JOIN users u    ON u.id = c.user_id
		 WHERE cr.status = $1
		 ORDER BY cr.created_at DESC`,
		status,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []*models.CommentReport
	for rows.Next() {
		var report models.CommentReport
		var comment models.Comment

		if err := rows.Scan(
			&report.ID,
			&report.CommentID,
			&report.ReportedBy,
			&report.Reason,
			&report.Status,
			&report.AdminNote,
			&report.CreatedAt,
			&comment.Text,
			&comment.UserID,
			&comment.Username,
		); err != nil {
			return nil, err
		}

		comment.ID = report.CommentID
		report.Comment = comment

		reports = append(reports, &report)
	}

	return reports, rows.Err()
}

// получает комментарий по ID
func (r *CommentRepository) GetCommentByID(id int64) (*models.Comment, error) {
	var comment models.Comment
	var username string

	err := r.db.QueryRow(
		`SELECT c.id, c.photo_id, c.user_id, c.text, c.likes_count, c.created_at
		 FROM comments c
		 WHERE c.id = $1`,
		id,
	).Scan(&comment.ID, &comment.PhotoID, &comment.UserID, &comment.Text, &comment.LikesCount, &comment.CreatedAt)

	if err != nil {
		return nil, err
	}

	err = r.db.QueryRow(`SELECT username FROM users WHERE id = $1`, comment.UserID).Scan(&username)
	if err == nil {
		comment.Username = username
	}

	return &comment, nil
}

// разрешает жалобу (админ)
func (r *CommentRepository) ResolveReport(reportID int64, action string, adminNote string) error {
	var commentID int64
	err := r.db.QueryRow(
		`SELECT comment_id FROM comment_reports WHERE id = $1`,
		reportID,
	).Scan(&commentID)
	if err != nil {
		return err
	}

	if action == "delete" {
		if _, err := r.db.Exec(`DELETE FROM comments WHERE id = $1`, commentID); err != nil {
			return err
		}
	}

	_, err = r.db.Exec(
		`UPDATE comment_reports
		    SET status = 'resolved',
		        admin_note = $1,
		        updated_at = NOW()
		  WHERE id = $2`,
		adminNote, reportID,
	)
	return err
}
