import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:pustaka/features/library/application/library_controller.dart';
import 'package:pustaka/features/library/data/library_repository.dart';
import 'package:pustaka/features/library/data/models/document.dart';
import 'package:pustaka/features/library/presentation/library_screen.dart';
import 'package:pustaka/features/library/presentation/widgets/document_card.dart';

class _FakeLib extends LibraryController {
  _FakeLib(this._d);
  final LibraryDocs _d;

  @override
  Future<LibraryDocs> build() async => _d;
}

Document _doc(String id, {bool owner = true}) => Document(
      id: id,
      title: id,
      mode: CaptureMode.photo,
      pageCount: 1,
      status: DocStatus.done,
      createdAt: DateTime(2026),
      isOwner: owner,
    );

Future<void> _pump(WidgetTester tester, LibraryDocs data) async {
  await tester.binding.setSurfaceSize(const Size(1000, 2000));
  addTearDown(() => tester.binding.setSurfaceSize(null));
  final router = GoRouter(
    routes: [
      GoRoute(path: '/', builder: (c, s) => const LibraryScreen()),
      GoRoute(
          path: '/doc/:id',
          builder: (c, s) =>
              Scaffold(body: Text('doc ${s.pathParameters['id']}'))),
      GoRoute(
          path: '/capture',
          builder: (c, s) => const Scaffold(body: Text('capture screen'))),
    ],
  );
  await tester.pumpWidget(
    ProviderScope(
      overrides: [libraryControllerProvider.overrideWith(() => _FakeLib(data))],
      child: MaterialApp.router(routerConfig: router),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('renders Mine + Shared sections with cards', (tester) async {
    await _pump(tester,
        (owned: [_doc('o1'), _doc('o2')], shared: [_doc('s1', owner: false)]));
    expect(find.text('Mine'), findsOneWidget);
    expect(find.text('Shared with me'), findsOneWidget);
    expect(find.byType(DocumentCard), findsNWidgets(3));
  });

  testWidgets('empty library shows New document CTA', (tester) async {
    await _pump(tester, (owned: <Document>[], shared: <Document>[]));
    expect(find.text('New document'), findsOneWidget);
  });

  testWidgets('tapping a card navigates to the reader', (tester) async {
    await _pump(tester, (owned: [_doc('o1')], shared: <Document>[]));
    await tester.tap(find.byType(DocumentCard).first);
    await tester.pumpAndSettle();
    expect(find.text('doc o1'), findsOneWidget);
  });
}
