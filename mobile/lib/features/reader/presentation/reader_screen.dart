import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../application/reader_controller.dart';
import 'widgets/outputs_tab.dart';
import 'widgets/page_view_pane.dart';

class ReaderScreen extends ConsumerStatefulWidget {
  const ReaderScreen({super.key, required this.docId});

  final String docId;

  @override
  ConsumerState<ReaderScreen> createState() => _ReaderScreenState();
}

class _ReaderScreenState extends ConsumerState<ReaderScreen> {
  bool _showImage = true;

  @override
  Widget build(BuildContext context) {
    final async = ref.watch(readerControllerProvider(widget.docId));
    return DefaultTabController(
      length: 2,
      child: Scaffold(
        appBar: AppBar(
          title: Text(async.valueOrNull?.doc.title ?? 'Document'),
          actions: [
            if (async.valueOrNull?.pages.any((p) => p.hasImage) ?? false)
              IconButton(
                tooltip: _showImage ? 'Show text' : 'Show image',
                icon: Icon(_showImage ? Icons.text_fields : Icons.image),
                onPressed: () => setState(() => _showImage = !_showImage),
              ),
            if (async.valueOrNull?.isOwner ?? false)
              IconButton(
                tooltip: 'Transform',
                icon: const Icon(Icons.auto_awesome),
                onPressed: () => context.go('/doc/${widget.docId}/transform'),
              ),
          ],
          bottom:
              const TabBar(tabs: [Tab(text: 'Pages'), Tab(text: 'Outputs')]),
        ),
        body: async.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => Center(child: Text('Failed to load: $e')),
          data: (state) => TabBarView(
            children: [
              state.pages.isEmpty
                  ? const Center(child: Text('No pages'))
                  : PageView.builder(
                      itemCount: state.pages.length,
                      itemBuilder: (context, i) => PageViewPane(
                        docId: widget.docId,
                        page: state.pages[i],
                        showImage: _showImage,
                      ),
                    ),
              OutputsTab(outputs: state.outputs),
            ],
          ),
        ),
      ),
    );
  }
}
