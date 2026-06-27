import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';

/// Thin wrapper over image_picker's camera capture.
class ImageCapture {
  ImageCapture([ImagePicker? picker]) : _picker = picker ?? ImagePicker();
  final ImagePicker _picker;

  Future<XFile?> takePhoto() =>
      _picker.pickImage(source: ImageSource.camera, imageQuality: 100);
}

final imageCaptureProvider = Provider<ImageCapture>((ref) => ImageCapture());
