import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pustaka/features/templates/application/templates_controller.dart';
import 'package:pustaka/features/templates/data/models/template.dart';
import 'package:pustaka/features/templates/presentation/templates_screen.dart';

class _FakeTemplates extends TemplatesController {
  _FakeTemplates(this._l);
  final List<Template> _l;

  @override
  Future<List<Template>> build() async => _l;
}

void main() {
  testWidgets('renders templates with scope and format chips', (tester) async {
    final templates = [
      const Template(
          id: 't1',
          name: 'Clean Markdown',
          scope: TemplateScope.document,
          outputFormat: OutputFormat.markdown,
          isBuiltin: true),
      const Template(
          id: 't2',
          name: 'Fields JSON',
          scope: TemplateScope.page,
          outputFormat: OutputFormat.json,
          isBuiltin: true),
    ];
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          templatesControllerProvider
              .overrideWith(() => _FakeTemplates(templates))
        ],
        child: const MaterialApp(home: TemplatesScreen()),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Clean Markdown'), findsOneWidget);
    expect(find.text('Fields JSON'), findsOneWidget);
    expect(find.text('document'), findsOneWidget);
    expect(find.text('json'), findsOneWidget);
  });
}
