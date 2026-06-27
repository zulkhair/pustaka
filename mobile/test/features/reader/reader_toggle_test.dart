import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pustaka/core/di/providers.dart';
import 'package:pustaka/features/library/data/models/document.dart';
import 'package:pustaka/features/reader/application/reader_controller.dart';
import 'package:pustaka/features/reader/data/models/page.dart';
import 'package:pustaka/features/reader/presentation/reader_screen.dart';
import 'package:pustaka/shared/widgets/network_image_auth.dart';

import '../../support/fake_api.dart';

class _FakeReader extends ReaderController {
  _FakeReader(this._s);
  final ReaderState _s;

  @override
  Future<ReaderState> build(String arg) async => _s;
}

Document _doc() => Document(
      id: 'd1',
      title: 'Doc',
      mode: CaptureMode.photo,
      pageCount: 1,
      status: DocStatus.done,
      createdAt: DateTime(2026),
      isOwner: true,
    );

Future<void> _pump(WidgetTester tester, ReaderState state) async {
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        apiClientProvider.overrideWithValue(apiClientReturningBytes(kTinyPng)),
        readerControllerProvider.overrideWith(() => _FakeReader(state)),
      ],
      child: const MaterialApp(home: ReaderScreen(docId: 'd1')),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('image page starts as image and toggles to OCR text',
      (tester) async {
    final state = ReaderState(
      doc: _doc(),
      pages: const [
        DocPage(
            pageNumber: 1,
            status: DocStatus.done,
            hasImage: true,
            ocrText: 'Body text',
            ocrStatus: DocStatus.done),
      ],
      outputs: const [],
      isOwner: true,
    );
    await _pump(tester, state);

    expect(find.byType(NetworkImageAuth), findsOneWidget);
    expect(find.text('Body text'), findsNothing);

    await tester.tap(find.byIcon(Icons.text_fields));
    await tester.pumpAndSettle();
    expect(find.text('Body text'), findsOneWidget);
  });

  testWidgets('text-mode page shows text directly with no toggle',
      (tester) async {
    final state = ReaderState(
      doc: _doc(),
      pages: const [
        DocPage(
            pageNumber: 1,
            status: DocStatus.done,
            hasImage: false,
            ocrText: 'Body text',
            ocrStatus: DocStatus.done),
      ],
      outputs: const [],
      isOwner: true,
    );
    await _pump(tester, state);

    expect(find.text('Body text'), findsOneWidget);
    expect(find.byIcon(Icons.text_fields), findsNothing);
    expect(find.byIcon(Icons.image), findsNothing);
  });

  testWidgets('non-owner reader hides Transform and Share actions',
      (tester) async {
    final state = ReaderState(
      doc: _doc(),
      pages: const [
        DocPage(
            pageNumber: 1,
            status: DocStatus.done,
            hasImage: false,
            ocrText: 'x',
            ocrStatus: DocStatus.done),
      ],
      outputs: const [],
      isOwner: false,
    );
    await _pump(tester, state);
    expect(find.byIcon(Icons.auto_awesome), findsNothing);
    expect(find.byIcon(Icons.share), findsNothing);
  });
}
