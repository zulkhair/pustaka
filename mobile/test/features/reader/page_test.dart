import 'package:flutter_test/flutter_test.dart';
import 'package:pustaka/features/library/data/models/document.dart';
import 'package:pustaka/features/reader/data/models/page.dart';

void main() {
  test('DocPage.fromJson maps fields and derives hasImage', () {
    final p = DocPage.fromJson({
      'pageNumber': 2,
      'status': 'done',
      'ocrText': '# hi',
      'ocrStatus': 'done',
      'imageUrl': '/api/documents/d1/pages/2/image',
    });
    expect(p.pageNumber, 2);
    expect(p.status, DocStatus.done);
    expect(p.hasImage, isTrue);
    expect(p.ocrText, '# hi');
    expect(p.ocrStatus, DocStatus.done);
  });

  test('DocPage without imageUrl has hasImage=false', () {
    final p = DocPage.fromJson({'pageNumber': 1, 'status': 'pending'});
    expect(p.hasImage, isFalse);
    expect(p.ocrText, '');
  });
}
