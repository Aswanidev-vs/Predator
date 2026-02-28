const fs = require('fs');
const path = require('path');

// Simple ICO file creation
// ICO format: header (6 bytes) + directory entries (16 bytes each) + image data

function createICO(inputPath, outputPath) {
    const pngBuffer = fs.readFileSync(inputPath);
    
    // ICO header
    const header = Buffer.alloc(6);
    header.writeUInt16LE(0, 0); // Reserved
    header.writeUInt16LE(1, 2); // Type: 1 = ICO
    header.writeUInt16LE(1, 4); // Count: 1 image
    
    // Directory entry
    const entry = Buffer.alloc(16);
    entry.writeUInt8(32, 0); // Width (0 means 256)
    entry.writeUInt8(32, 1); // Height (0 means 256)
    entry.writeUInt8(0, 2);  // Color palette
    entry.writeUInt8(0, 3); // Reserved
    entry.writeUInt16LE(1, 4); // Color planes
    entry.writeUInt16LE(32, 6); // Bits per pixel
    entry.writeUInt32LE(pngBuffer.length, 8); // Size of image data
    entry.writeUInt32LE(22, 12); // Offset to image data (6 + 16 = 22)
    
    // Combine all parts
    const icoBuffer = Buffer.concat([header, entry, pngBuffer]);
    
    fs.writeFileSync(outputPath, icoBuffer);
    console.log(`Created ICO: ${outputPath} (${icoBuffer.length} bytes)`);
}

// Use logov4.png from parent directory
const inputFile = process.argv[2] || '../logov4.png';
const outputFile = process.argv[3] || './build/windows/icon.ico';

try {
    createICO(inputFile, outputFile);
    console.log('Icon conversion successful!');
} catch (err) {
    console.error('Error:', err.message);
    process.exit(1);
}
