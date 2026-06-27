import 'package:flutter_test/flutter_test.dart';
import 'package:pustaka/features/templates/data/models/template.dart';

void main() {
  test('Template.fromJson maps scope, outputFormat, isBuiltin', () {
    final t = Template.fromJson({
      'id': 't1',
      'name': 'Clean Markdown',
      'scope': 'document',
      'outputFormat': 'markdown',
      'isBuiltin': true,
    });
    expect(t.scope, TemplateScope.document);
    expect(t.outputFormat, OutputFormat.markdown);
    expect(t.isBuiltin, isTrue);
  });

  test('unknown enums fall back to page/markdown', () {
    final t = Template.fromJson(
        {'id': 't', 'name': 'x', 'scope': 'weird', 'outputFormat': 'weird'});
    expect(t.scope, TemplateScope.page);
    expect(t.outputFormat, OutputFormat.markdown);
  });
}
