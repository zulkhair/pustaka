import 'dart:typed_data';

import 'package:flutter_image_compress/flutter_image_compress.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image/image.dart' as img;

const _maxEdge = 2048;
const _quality = 80;

/// Pure-Dart compression (no platform channel): decode → cap longest edge at
/// 2048 → re-encode JPEG q80. Used by tests and the pure-Dart compressor.
Uint8List compressBytes(Uint8List input) {
  final decoded = img.decodeImage(input);
  if (decoded == null) return input;
  final longest =
      decoded.width >= decoded.height ? decoded.width : decoded.height;
  var out = decoded;
  if (longest > _maxEdge) {
    out = decoded.width >= decoded.height
        ? img.copyResize(decoded, width: _maxEdge)
        : img.copyResize(decoded, height: _maxEdge);
  }
  return Uint8List.fromList(img.encodeJpg(out, quality: _quality));
}

/// Injectable compression seam so the capture controller stays off the platform
/// channel in tests.
abstract class ImageCompressor {
  Future<Uint8List> compress(Uint8List input);
}

/// On-device default — fast native compression.
class FlutterImageCompressor implements ImageCompressor {
  @override
  Future<Uint8List> compress(Uint8List input) {
    return FlutterImageCompress.compressWithList(
      input,
      minWidth: _maxEdge,
      minHeight: _maxEdge,
      quality: _quality,
    );
  }
}

/// Pure-Dart implementation, injected by tests.
class PureDartImageCompressor implements ImageCompressor {
  @override
  Future<Uint8List> compress(Uint8List input) async => compressBytes(input);
}

final imageCompressorProvider =
    Provider<ImageCompressor>((ref) => FlutterImageCompressor());
