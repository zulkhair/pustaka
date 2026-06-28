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
                  _Grid(docs: docs.owned),
                ],
                if (docs.shared.isNotEmpty) ...[
                  const _Header('Shared with me'),
                  _Grid(docs: docs.shared),
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

class _Grid extends StatelessWidget {
  const _Grid({required this.docs});
  final List<Document> docs;

  @override
  Widget build(BuildContext context) {
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
                doc: d, onTap: () => context.push('/doc/${d.id}'));
          },
          childCount: docs.length,
        ),
      ),
    );
  }
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
