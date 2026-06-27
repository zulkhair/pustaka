import 'package:flutter_test/flutter_test.dart';
import 'package:pustaka/features/library/data/models/document.dart';

void main() {
  test('fromJson maps fields, enums, date, thumb; isOwner defaults false', () {
    final d = Document.fromJson({
      'id': 'd1',
      'title': 'Doc',
      'mode': 'text',
      'pageCount': 3,
      'status': 'done',
      'createdAt': '2026-06-27T10:00:00Z',
      'thumbUrl': '/api/documents/d1/pages/1/thumb',
    });
    expect(d.id, 'd1');
    expect(d.mode, CaptureMode.text);
    expect(d.pageCount, 3);
    expect(d.status, DocStatus.done);
    expect(d.createdAt.year, 2026);
    expect(d.thumbUrl, '/api/documents/d1/pages/1/thumb');
    expect(d.isOwner, isFalse);
  });

  test('unknown status falls back to failed; mode defaults to photo', () {
    final d =
        Document.fromJson({'id': 'x', 'status': 'weird', 'mode': 'weird'});
    expect(d.status, DocStatus.failed);
    expect(d.mode, CaptureMode.photo);
  });

  test('copyWith sets isOwner', () {
    final d = Document.fromJson({'id': 'x'}).copyWith(isOwner: true);
    expect(d.isOwner, isTrue);
  });
}
