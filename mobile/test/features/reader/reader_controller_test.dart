import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:pustaka/features/library/application/library_controller.dart';
import 'package:pustaka/features/library/data/library_repository.dart';
import 'package:pustaka/features/library/data/models/document.dart';
import 'package:pustaka/features/reader/application/reader_controller.dart';
import 'package:pustaka/features/reader/data/models/page.dart';
import 'package:pustaka/features/reader/data/reader_repository.dart';
import 'package:pustaka/features/transform/data/models/output.dart';

class MockReaderRepository extends Mock implements ReaderRepository {}

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

void main() {
  test('build returns doc/pages/outputs and isOwner from the library',
      () async {
    final doc = _doc('d1');
    final repo = MockReaderRepository();
    when(() => repo.fetchDoc('d1')).thenAnswer((_) async => (
          doc: doc,
          pages: [
            const DocPage(
              pageNumber: 1,
              status: DocStatus.done,
              hasImage: true,
              ocrText: 'body',
              ocrStatus: DocStatus.done,
            ),
          ],
          outputs: [
            Output(
              id: 'out1',
              documentId: 'd1',
              templateId: 't1',
              content: 'x',
              model: 'm',
              status: DocStatus.done,
              createdAt: DateTime(2026),
            ),
          ],
        ));

    final c = ProviderContainer(overrides: [
      readerRepositoryProvider.overrideWithValue(repo),
      libraryControllerProvider
          .overrideWith(() => _FakeLib((owned: [doc], shared: <Document>[]))),
    ]);
    addTearDown(c.dispose);

    await c.read(libraryControllerProvider.future);
    final state = await c.read(readerControllerProvider('d1').future);
    expect(state.pages, hasLength(1));
    expect(state.outputs, hasLength(1));
    expect(state.isOwner, isTrue);
  });
}
