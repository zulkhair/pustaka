import 'package:flutter_test/flutter_test.dart';
import 'package:pustaka/features/capture/data/models/page_upload_result.dart';

void main() {
  test('PageUploadResult.fromJson maps pageNumber and ocrText', () {
    final r = PageUploadResult.fromJson({'pageNumber': 3, 'ocrText': 'Hello'});
    expect(r.pageNumber, 3);
    expect(r.ocrText, 'Hello');
  });
}
