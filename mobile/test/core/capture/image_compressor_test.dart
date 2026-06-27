import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:image/image.dart' as img;
import 'package:pustaka/core/capture/image_compressor.dart';

void main() {
  test('compressBytes caps the longest edge at 2048 and shrinks the bytes', () {
    final big = img.Image(width: 2400, height: 1800);
    for (var y = 0; y < big.height; y++) {
      for (var x = 0; x < big.width; x++) {
        big.setPixelRgb(x, y, x % 256, y % 256, (x + y) % 256);
      }
    }
    final input = Uint8List.fromList(img.encodeJpg(big, quality: 95));

    final out = compressBytes(input);
    final decoded = img.decodeImage(out)!;

    expect(decoded.width, lessThanOrEqualTo(2048));
    expect(decoded.height, lessThanOrEqualTo(2048));
    expect(decoded.width, 2048); // longest edge scaled down to the cap
    expect(out.length, lessThan(input.length));
  });
}
