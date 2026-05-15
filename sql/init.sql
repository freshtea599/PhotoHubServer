-- =====================================================
-- 1. Таблицы
-- =====================================================

CREATE TABLE public.users (
    id integer NOT NULL,
    email character varying(255) NOT NULL,
    password_hash character varying(255) NOT NULL,
    username character varying(255) NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    is_admin boolean DEFAULT false
);

CREATE SEQUENCE public.users_id_seq AS integer START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.users_id_seq OWNED BY public.users.id;
ALTER TABLE ONLY public.users ALTER COLUMN id SET DEFAULT nextval('public.users_id_seq'::regclass);

CREATE TABLE public.photos (
    id integer NOT NULL,
    user_id integer NOT NULL,
    url character varying(500) NOT NULL,
    file_path character varying(500) NOT NULL,
    file_size bigint,
    mime_type character varying(50),
    description text,
    is_public boolean DEFAULT false,
    blurhash text,
    content_hash text,
    width integer,
    height integer,
    likes_count integer DEFAULT 0,
    comments_count integer DEFAULT 0,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE public.photos_id_seq AS integer START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.photos_id_seq OWNED BY public.photos.id;
ALTER TABLE ONLY public.photos ALTER COLUMN id SET DEFAULT nextval('public.photos_id_seq'::regclass);

CREATE TABLE public.photo_variants (
    id integer NOT NULL,
    photo_id integer NOT NULL,
    size_name character varying(50) NOT NULL,
    format character varying(10) NOT NULL,
    file_path character varying(500) NOT NULL,
    file_size bigint,
    width integer,
    height integer,
    quality integer,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE public.photo_variants_id_seq AS integer START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.photo_variants_id_seq OWNED BY public.photo_variants.id;
ALTER TABLE ONLY public.photo_variants ALTER COLUMN id SET DEFAULT nextval('public.photo_variants_id_seq'::regclass);

CREATE TABLE public.photo_likes (
    id integer NOT NULL,
    photo_id integer NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE public.photo_likes_id_seq AS integer START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.photo_likes_id_seq OWNED BY public.photo_likes.id;
ALTER TABLE ONLY public.photo_likes ALTER COLUMN id SET DEFAULT nextval('public.photo_likes_id_seq'::regclass);

CREATE TABLE public.comments (
    id integer NOT NULL,
    photo_id integer NOT NULL,
    user_id integer NOT NULL,
    text text NOT NULL,
    likes_count integer DEFAULT 0,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE public.comments_id_seq AS integer START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.comments_id_seq OWNED BY public.comments.id;
ALTER TABLE ONLY public.comments ALTER COLUMN id SET DEFAULT nextval('public.comments_id_seq'::regclass);

CREATE TABLE public.comment_likes (
    id integer NOT NULL,
    comment_id integer NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE public.comment_likes_id_seq AS integer START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.comment_likes_id_seq OWNED BY public.comment_likes.id;
ALTER TABLE ONLY public.comment_likes ALTER COLUMN id SET DEFAULT nextval('public.comment_likes_id_seq'::regclass);

CREATE TABLE public.comment_reports (
    id integer NOT NULL,
    comment_id integer NOT NULL,
    reported_by integer NOT NULL,
    reason character varying(255),
    status character varying(20) DEFAULT 'pending'::character varying,
    admin_note text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE public.comment_reports_id_seq AS integer START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.comment_reports_id_seq OWNED BY public.comment_reports.id;
ALTER TABLE ONLY public.comment_reports ALTER COLUMN id SET DEFAULT nextval('public.comment_reports_id_seq'::regclass);

-- =====================================================
-- 2. Первичные ключи и уникальность
-- =====================================================

ALTER TABLE ONLY public.users ADD CONSTRAINT users_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.users ADD CONSTRAINT users_email_key UNIQUE (email);
ALTER TABLE ONLY public.photos ADD CONSTRAINT photos_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.photo_variants ADD CONSTRAINT photo_variants_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.photo_variants ADD CONSTRAINT photo_variants_photo_id_size_name_format_key UNIQUE (photo_id, size_name, format);
ALTER TABLE ONLY public.photo_likes ADD CONSTRAINT photo_likes_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.photo_likes ADD CONSTRAINT photo_likes_photo_id_user_id_key UNIQUE (photo_id, user_id);
ALTER TABLE ONLY public.comments ADD CONSTRAINT comments_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.comment_likes ADD CONSTRAINT comment_likes_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.comment_likes ADD CONSTRAINT comment_likes_comment_id_user_id_key UNIQUE (comment_id, user_id);
ALTER TABLE ONLY public.comment_reports ADD CONSTRAINT comment_reports_pkey PRIMARY KEY (id);

-- =====================================================
-- 3. Индексы
-- =====================================================

CREATE INDEX idx_users_email ON public.users USING btree (email);
CREATE INDEX idx_photos_created_at ON public.photos USING btree (created_at);
CREATE INDEX idx_photos_user_id ON public.photos USING btree (user_id);
CREATE INDEX idx_photos_user_public ON public.photos USING btree (user_id, is_public);
CREATE INDEX idx_photos_likes_count ON public.photos USING btree (likes_count);
CREATE INDEX idx_photos_comments_count ON public.photos USING btree (comments_count);
CREATE INDEX idx_photo_variants_photo_id ON public.photo_variants USING btree (photo_id);
CREATE INDEX idx_photo_likes_photo_id ON public.photo_likes USING btree (photo_id);
CREATE INDEX idx_photo_likes_user_id ON public.photo_likes USING btree (user_id);
CREATE INDEX idx_comments_photo_id ON public.comments USING btree (photo_id);
CREATE INDEX idx_comments_user_id ON public.comments USING btree (user_id);
CREATE INDEX idx_comment_likes_comment_id ON public.comment_likes USING btree (comment_id);
CREATE INDEX idx_comment_likes_user_id ON public.comment_likes USING btree (user_id);
CREATE INDEX idx_comment_reports_status ON public.comment_reports USING btree (status);

-- =====================================================
-- 4. Внешние ключи
-- =====================================================

ALTER TABLE ONLY public.photos ADD CONSTRAINT photos_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.photo_variants ADD CONSTRAINT photo_variants_photo_id_fkey FOREIGN KEY (photo_id) REFERENCES public.photos(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.photo_likes ADD CONSTRAINT photo_likes_photo_id_fkey FOREIGN KEY (photo_id) REFERENCES public.photos(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.photo_likes ADD CONSTRAINT photo_likes_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.comments ADD CONSTRAINT comments_photo_id_fkey FOREIGN KEY (photo_id) REFERENCES public.photos(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.comments ADD CONSTRAINT comments_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.comment_likes ADD CONSTRAINT comment_likes_comment_id_fkey FOREIGN KEY (comment_id) REFERENCES public.comments(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.comment_likes ADD CONSTRAINT comment_likes_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.comment_reports ADD CONSTRAINT comment_reports_comment_id_fkey FOREIGN KEY (comment_id) REFERENCES public.comments(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.comment_reports ADD CONSTRAINT comment_reports_reported_by_fkey FOREIGN KEY (reported_by) REFERENCES public.users(id) ON DELETE CASCADE;