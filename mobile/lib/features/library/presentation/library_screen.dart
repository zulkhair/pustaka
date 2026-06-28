import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../application/library_controller.dart';
import '../data/models/document.dart';
import 'widgets/document_card.dart';

class LibraryScreen extends ConsumerWidget {
  const LibraryScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final async = ref.watch(libraryControllerProvider);
    return Scaffold(
      appBar: AppBar(title: const Text('Library')),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () => context.push('/capture'),
        icon: const Icon(Icons.add_a_photo_outlined),
        label: const Text('New'),
      ),
      body: async.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) => Center(child: Text('Failed to load: $e')),
        data: (docs) {
          if (docs.owned.isEmpty && docs.shared.isEmpty) {
            return _EmptyState(onCreate: () => context.push('/capture'));
          }
          return RefreshIndicator(
            onRefresh: () =>
                ref.read(libraryControllerProvider.notifier).refresh(),
            child: CustomScrollView(
              slivers: [
                if (docs.owned.isNotEmpty) ...[
                  const _Header('Mine'),
                  _Grid(docs: docs.owned, owned: true),
                ],
                if (docs.shared.isNotEmpty) ...[
                  const _Header('Shared with me'),
                  _Grid(docs: docs.shared, owned: false),
                ],
              ],
            ),
          );
        },
      ),
    );
  }
}

class _Header extends StatelessWidget {
  const _Header(this.title);
  final String title;

  @override
  Widget build(BuildContext context) {
    return SliverToBoxAdapter(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
        child: Text(title, style: Theme.of(context).textTheme.titleMedium),
      ),
    );
  }
}

class _Grid extends ConsumerWidget {
  const _Grid({required this.docs, required this.owned});
  final List<Document> docs;
  final bool owned;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return SliverPadding(
      padding: const EdgeInsets.symmetric(horizontal: 12),
      sliver: SliverGrid(
        gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
          crossAxisCount: 2,
          childAspectRatio: 0.72,
          crossAxisSpacing: 8,
          mainAxisSpacing: 8,
        ),
        delegate: SliverChildBuilderDelegate(
          (context, i) {
            final d = docs[i];
            return DocumentCard(
              doc: d,
              onTap: () => context.push('/doc/${d.id}'),
              onResume: owned ? () => context.push('/doc/${d.id}/capture') : null,
              onRename: owned ? () => _promptRename(context, ref, d) : null,
              onDelete: owned ? () => _confirmDelete(context, ref, d) : null,
            );
          },
          childCount: docs.length,
        ),
      ),
    );
  }
}

Future<void> _promptRename(
    BuildContext context, WidgetRef ref, Document doc) async {
  final controller = TextEditingController(text: doc.title);
  final title = await showDialog<String>(
    context: context,
    builder: (ctx) => AlertDialog(
      title: const Text('Rename document'),
      content: TextField(
        controller: controller,
        autofocus: true,
        decoration: const InputDecoration(labelText: 'Title'),
        onSubmitted: (v) => Navigator.of(ctx).pop(v.trim()),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(ctx).pop(),
          child: const Text('Cancel'),
        ),
        FilledButton(
          onPressed: () => Navigator.of(ctx).pop(controller.text.trim()),
          child: const Text('Save'),
        ),
      ],
    ),
  );
  if (title == null || title.isEmpty || title == doc.title) return;
  await ref.read(libraryControllerProvider.notifier).rename(doc.id, title);
}

Future<void> _confirmDelete(
    BuildContext context, WidgetRef ref, Document doc) async {
  final ok = await showDialog<bool>(
    context: context,
    builder: (ctx) => AlertDialog(
      title: const Text('Delete document'),
      content: Text('Delete "${doc.title}"? You can ask an admin to restore it.'),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(ctx).pop(false),
          child: const Text('Cancel'),
        ),
        FilledButton.tonal(
          onPressed: () => Navigator.of(ctx).pop(true),
          child: const Text('Delete'),
        ),
      ],
    ),
  );
  if (ok != true) return;
  await ref.read(libraryControllerProvider.notifier).delete(doc.id);
}

class _EmptyState extends StatelessWidget {
  const _EmptyState({required this.onCreate});
  final VoidCallback onCreate;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.library_books_outlined, size: 64),
          const SizedBox(height: 12),
          const Text('No documents yet'),
          const SizedBox(height: 12),
          FilledButton(onPressed: onCreate, child: const Text('New document')),
        ],
      ),
    );
  }
}
