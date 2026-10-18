import { mkdirSync, writeFileSync } from 'node:fs'
import { deflateSync } from 'node:zlib'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..', 'public')
mkdirSync(root, { recursive: true })
const table = Array.from({ length: 256 }, (_, n) => {
  let c = n
  for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1
  return c >>> 0
})
function crc32(buffer) {
  let c = 0xffffffff
  for (const byte of buffer) c = table[(c ^ byte) & 255] ^ (c >>> 8)
  return (c ^ 0xffffffff) >>> 0
}
function chunk(kind, data) {
  const type = Buffer.from(kind, 'ascii')
  const length = Buffer.alloc(4)
  length.writeUInt32BE(data.length)
  const crc = Buffer.alloc(4)
  crc.writeUInt32BE(crc32(Buffer.concat([type, data])))
  return Buffer.concat([length, type, data, crc])
}
function ellipse(x, y, cx, cy, rx, ry, angle = 0) {
  const c = Math.cos(angle), s = Math.sin(angle)
  const a = (x - cx) * c + (y - cy) * s
  const b = -(x - cx) * s + (y - cy) * c
  return a * a / (rx * rx) + b * b / (ry * ry) <= 1
}
function sample(x, y) {
  let color = [56, 104, 78]
  if (x >= 90 && x <= 102 && y >= 77 && y <= 151) color = [238, 242, 218]
  if (ellipse(x, y, 76, 118, 28, 13, .65) || ellipse(x, y, 116, 102, 30, 14, -.6)) color = [185, 216, 156]
  if (ellipse(x, y, 96, 48, 15, 23) || ellipse(x, y, 119, 67, 22, 15) || ellipse(x, y, 96, 86, 15, 23) || ellipse(x, y, 73, 67, 22, 15)) color = [247, 231, 196]
  if (ellipse(x, y, 96, 67, 12, 12)) color = [226, 183, 91]
  return color
}
function make(size) {
  const stride = 1 + size * 4
  const data = Buffer.alloc(stride * size)
  for (let y = 0; y < size; y++) {
    const row = y * stride
    data[row] = 0
    for (let x = 0; x < size; x++) {
      const accum = [0, 0, 0]
      for (let sy = 0; sy < 2; sy++) for (let sx = 0; sx < 2; sx++) {
        const p = sample((x + (sx + .5) / 2) * 192 / size, (y + (sy + .5) / 2) * 192 / size)
        for (let i = 0; i < 3; i++) accum[i] += p[i]
      }
      const offset = row + 1 + x * 4
      for (let i = 0; i < 3; i++) data[offset + i] = Math.round(accum[i] / 4)
      data[offset + 3] = 255
    }
  }
  const header = Buffer.alloc(13)
  header.writeUInt32BE(size, 0)
  header.writeUInt32BE(size, 4)
  header[8] = 8
  header[9] = 6
  return Buffer.concat([
    Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    chunk('IHDR', header),
    chunk('IDAT', deflateSync(data, { level: 9 })),
    chunk('IEND', Buffer.alloc(0)),
  ])
}
for (const size of [192, 512]) {
  writeFileSync(join(root, 'icon-' + size + '.png'), make(size))
  console.log('Generated icon-' + size + '.png')
}
