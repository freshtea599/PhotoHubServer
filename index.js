const express = require("express");
const cors = require("cors");
const fs = require("fs");
const path = require("path");
const multer = require("multer");
const sharp = require("sharp");

const app = express();
const PORT = 3000;
const usersFilePath = path.join(__dirname, "users.json");
const uploadDir = path.join(__dirname, "public/uploads");

// Создаем папку, если её нет
if (!fs.existsSync(uploadDir)) fs.mkdirSync(uploadDir, { recursive: true });

app.use(cors());
app.use(express.json());
app.use("/uploads", express.static(uploadDir));

// Настройка загрузки файлов
const storage = multer.diskStorage({
  destination: (req, file, cb) => cb(null, uploadDir),
  filename: (req, file, cb) => cb(null, Date.now() + path.extname(file.originalname)),
});
const upload = multer({ storage });

// Проверка работы API
app.get("/api", (req, res) => {
  res.send("API работает! Доступные маршруты: /api/login, /api/register, /api/photos, /api/upload");
});

// Логин пользователя
app.post("/api/login", (req, res) => {
  const { email, password } = req.body;

  if (!fs.existsSync(usersFilePath)) {
    return res.status(500).json({ message: "База данных пользователей отсутствует" });
  }

  const usersData = JSON.parse(fs.readFileSync(usersFilePath, "utf-8")) || [];
  const user = usersData.find(user => user.email === email && user.password === password);

  if (user) {
    return res.json({ token: "test-token", user: user.email });
  }

  res.status(401).json({ message: "Неверный логин или пароль" });
});

// Регистрация пользователя
app.post("/api/register", (req, res) => {
  const { email, password } = req.body;

  if (!email || !password) {
    return res.status(400).json({ message: "Заполните все поля!" });
  }

  let usersData = [];

  if (fs.existsSync(usersFilePath)) {
    usersData = JSON.parse(fs.readFileSync(usersFilePath, "utf-8"));
  }

  if (usersData.find(user => user.email === email)) {
    return res.status(400).json({ message: "Пользователь уже существует" });
  }

  usersData.push({ email, password });
  fs.writeFileSync(usersFilePath, JSON.stringify(usersData, null, 2));

  res.status(201).json({ message: "Пользователь успешно зарегистрирован" });
});

// Загрузка изображений с сжатием
app.post("/api/upload", upload.single("image"), async (req, res) => {
  if (!req.file) {
    return res.status(400).json({ message: "Файл не загружен" });
  }

  const filePath = path.join(uploadDir, req.file.filename);
  const ext = path.extname(req.file.filename).toLowerCase();

  try {
    if (ext === ".jpeg" || ext === ".jpg") {
      await sharp(filePath).jpeg({ quality: 70 }).toFile(filePath + "_compressed");
    } else if (ext === ".png") {
      await sharp(filePath).png({ compressionLevel: 9 }).toFile(filePath + "_compressed");
    }

    fs.unlinkSync(filePath); // Удаляем оригинальный файл
    fs.renameSync(filePath + "_compressed", filePath); // Переименовываем сжатый файл

    res.json({ message: "Файл загружен и сжат", url: `/uploads/${req.file.filename}` });
  } catch (error) {
    res.status(500).json({ message: "Ошибка обработки файла", error: error.message });
  }
});

// Получение списка фотографий
app.get("/api/photos", (req, res) => {
  fs.readdir(uploadDir, (err, files) => {
    if (err) {
      return res.status(500).json({ message: "Ошибка чтения файлов" });
    }

    const imageFiles = files.filter(file => /\.(jpeg|jpg|png|webp)$/i.test(file));
    const urlsFiles = imageFiles.map(file => ({ url: `/uploads/${file}` }));

    res.json(urlsFiles);
  });
});

// Запуск сервера
app.listen(PORT, () => {
  console.log(`Сервер запущен на порту ${PORT}`);
});
