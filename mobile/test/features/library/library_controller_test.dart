import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:pustaka/features/library/application/library_controller.dart';
import 'package:pustaka/features/library/data/library_repository.dart';
import 'package:pustaka/features/library/data/models/document.dart';

class MockLibraryRepository extends Mock implements LibraryRepository {}

Document _doc(String id, {bool owner = true}) => Document(
      id: id,
      title: id,
      mode: CaptureMode.photo,
      pageCount: 0,
      status: DocStatus.pending,
      createdAt: DateTime(2026),
      isOwner: owner,
    );

void main() {
  setUpAll(() => registerFallbackValue(CaptureMode.photo));

  test('build exposes owned + shared from fetch', () async {
    final repo = MockLibraryRepository();
    when(repo.fetch).thenAnswer(
        (_) async => (owned: [_doc('o1')], shared: [_doc('s1', owner: false)]));
    final c = ProviderContainer(
        overrides: [libraryRepositoryProvider.overrideWithValue(repo)]);
    addTearDown(c.dispose);

    final docs = await c.read(libraryControllerProvider.future);
    expect(docs.owned.single.id, 'o1');
    expect(docs.shared.single.id, 's1');
  });

  test('createDocument prepends to owned', () async {
    final repo = MockLibraryRepository();
    when(repo.fetch)
        .thenAnswer((_) async => (owned: <Document>[], shared: <Document>[]));
    when(() => repo.createDocument(any(), any()))
        .thenAnswer((_) async => _doc('new'));
    final c = ProviderContainer(
        overrides: [libraryRepositoryProvider.overrideWithValue(repo)]);
    addTearDown(c.dispose);

    await c.read(libraryControllerProvider.future);
    final created = await c
        .read(libraryControllerProvider.notifier)
        .createDocument('T', CaptureMode.photo);
    expect(created.id, 'new');
    expect(
        c.read(libraryControllerProvider).valueOrNull!.owned.first.id, 'new');
  });

  test('rename replaces the owned doc in place', () async {
    final repo = MockLibraryRepository();
    when(repo.fetch)
        .thenAnswer((_) async => (owned: [_doc('o1')], shared: <Document>[]));
    when(() => repo.renameDocument('o1', 'Renamed')).thenAnswer(
        (_) async => Document(
              id: 'o1',
              title: 'Renamed',
              mode: CaptureMode.photo,
              pageCount: 0,
              status: DocStatus.pending,
              createdAt: DateTime(2026),
              isOwner: true,
            ));
    final c = ProviderContainer(
        overrides: [libraryRepositoryProvider.overrideWithValue(repo)]);
    addTearDown(c.dispose);

    await c.read(libraryControllerProvider.future);
    await c.read(libraryControllerProvider.notifier).rename('o1', 'Renamed');
    final owned = c.read(libraryControllerProvider).valueOrNull!.owned;
    expect(owned.single.id, 'o1');
    expect(owned.single.title, 'Renamed');
  });

  test('delete removes the doc from owned', () async {
    final repo = MockLibraryRepository();
    when(repo.fetch).thenAnswer(
        (_) async => (owned: [_doc('o1'), _doc('o2')], shared: <Document>[]));
    when(() => repo.deleteDocument('o1')).thenAnswer((_) async {});
    final c = ProviderContainer(
        overrides: [libraryRepositoryProvider.overrideWithValue(repo)]);
    addTearDown(c.dispose);

    await c.read(libraryControllerProvider.future);
    await c.read(libraryControllerProvider.notifier).delete('o1');
    final owned = c.read(libraryControllerProvider).valueOrNull!.owned;
    expect(owned.map((d) => d.id), ['o2']);
  });
}
