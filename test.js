const sharp = require('sharp');
const fs = require('fs');
const path = require('path');

const inputDir = 'public/uploads'; // Входная папка
const outputDir = 'public/output'; // Выходная папка

if (!fs.existsSync(outputDir)) {
    fs.mkdirSync(outputDir, { recursive: true });
}

fs.readdir(inputDir, (err, files) => {
    if (err) {
        console.error('Ошибка при чтении директории:', err);
        return;
    }

    files.forEach(file => {
        const inputPath = path.join(inputDir, file);
        const outputPath = path.join(outputDir, file);
        const ext = path.extname(file).toLowerCase();

        if (ext === '.jpg' || ext === '.jpeg') {
            sharp(inputPath)
                .jpeg({ quality: 70 })
                .toFile(outputPath)
                .then(() => console.log(`Сжато: ${file} (JPEG)`))
                .catch(err => console.error(`Ошибка обработки ${file}:`, err));
        } else if (ext === '.png') {
            sharp(inputPath)    
                .png({ compressionLevel: 1 })
                .toFile(outputPath)
                .then(() => console.log(`Сжато: ${file} (PNG)`))
                .catch(err => console.error(`Ошибка обработки ${file}:`, err));
        } else if (ext === '.webp') {
            sharp(inputPath)
                .webp({ quality: 75 })
                .toFile(outputPath)
                .then(() => console.log(`Сжато: ${file} (WebP)`))
                .catch(err => console.error(`Ошибка обработки ${file}:`, err));
        } else {
            console.log(`Пропущен файл: ${file} (неподдерживаемый формат)`);
        }
    });
});
