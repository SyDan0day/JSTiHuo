/**
 * solveChallenge 用途：读取本地七段数字挑战的像素，仅用于隔离浏览器验收。
 * 参数：page 浏览器页面；locator 当前挑战图片定位器。
 * 返回值：四位答案 Promise，答案仅驻留内存，不打印或写文件。
 */
async function solveChallenge(page, locator) {
  await locator.waitFor();
  return locator.evaluate(async (image) => {
    await image.decode();
    const canvas = document.createElement('canvas');
    canvas.width = image.naturalWidth;
    canvas.height = image.naturalHeight;
    const context = canvas.getContext('2d');
    context.drawImage(image, 0, 0);
    const segments = [
      'abcdef',
      'bc',
      'abdeg',
      'abcdg',
      'bcfg',
      'acdfg',
      'acdefg',
      'abc',
      'abcdefg',
      'abcdfg',
    ];
    const sample = {
      a: [11, 3],
      b: [21, 12],
      c: [21, 30],
      d: [11, 39],
      e: [1, 30],
      f: [1, 12],
      g: [11, 21],
    };
    let answer = '';
    for (let i = 0; i < 4; i++) {
      const mask = Object.entries(sample)
        .filter(([, [x, y]]) => context.getImageData(12 + i * 34 + x, 8 + y, 1, 1).data[0] < 100)
        .map(([name]) => name)
        .join('');
      const digit = segments.indexOf(mask);
      if (digit < 0) throw new Error('本地挑战图形与夹具不符');
      answer += digit;
    }
    return answer;
  });
}
module.exports = { solveChallenge };
