import 'package:flutter_test/flutter_test.dart';
import 'package:pustaka/features/library/data/models/document.dart';
import 'package:pustaka/features/transform/data/models/output.dart';

void main() {
  test('Output.fromJson maps content, status, createdAt', () {
    final o = Output.fromJson({
      'id': 'out1',
      'documentId': 'd1',
      'templateId': 't1',
      'content': '[{"page_number":1,"result":{}}]',
      'model': 'qwen',
      'status': 'done',
      'createdAt': '2026-06-27T10:00:00Z',
    });
    expect(o.id, 'out1');
    expect(o.content, '[{"page_number":1,"result":{}}]');
    expect(o.status, DocStatus.done);
    expect(o.createdAt.year, 2026);
  });
}
