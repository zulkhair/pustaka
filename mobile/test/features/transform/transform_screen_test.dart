import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:pustaka/features/library/data/models/document.dart';
import 'package:pustaka/features/templates/application/templates_controller.dart';
import 'package:pustaka/features/templates/data/models/template.dart';
import 'package:pustaka/features/transform/data/models/output.dart';
import 'package:pustaka/features/transform/data/transform_repository.dart';
import 'package:pustaka/features/transform/presentation/transform_screen.dart';

class MockTransformRepository extends Mock implements TransformRepository {}

class _FakeTemplates extends TemplatesController {
  _FakeTemplates(this._l);
  final List<Template> _l;

  @override
  Future<List<Template>> build() async => _l;
}

void main() {
  testWidgets('select a template, run, and render the output', (tester) async {
    final repo = MockTransformRepository();
    when(() => repo.run(any(), any())).thenAnswer((_) async => Output(
          id: 'out1',
          documentId: 'd1',
          templateId: 't1',
          content: '# transformed',
          model: 'qwen',
          status: DocStatus.done,
          createdAt: DateTime(2026),
        ));

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
              .overrideWith(() => _FakeTemplates(templates)),
          transformRepositoryProvider.overrideWithValue(repo),
        ],
        child: const MaterialApp(home: TransformScreen(docId: 'd1')),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Clean Markdown'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Run'));
    await tester.pumpAndSettle();

    expect(find.text('# transformed'), findsOneWidget);
  });
}
