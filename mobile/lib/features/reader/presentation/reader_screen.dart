import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../library/application/library_controller.dart';
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
  int _currentIndex = 0;

  /// Local echo of the cover page so the marker updates instantly after a set,
  /// without re-fetching the whole document. Null until the user changes it.
  int? _coverOverride;

  Future<void> _setCover(int pageNumber) async {
    try {
      await ref
          .read(libraryControllerProvider.notifier)
          .setThumbnail(widget.docId, pageNumber);
      if (!mounted) return;
      setState(() => _coverOverride = pageNumber);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Page $pageNumber set as cover')),
      );
    } catch (_) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Could not set cover')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final async = ref.watch(readerControllerProvider(widget.docId));
    final state = async.valueOrNull;
    final pages = state?.pages ?? const [];
    final canSetCover = (state?.isOwner ?? false) &&
        _currentIndex < pages.length &&
        pages[_currentIndex].hasImage;
    final coverPage = _coverOverride ?? state?.doc.thumbPage ?? 1;
    final currentIsCover = _currentIndex < pages.length &&
        pages[_currentIndex].pageNumber == coverPage;
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
            if (canSetCover)
              IconButton(
                tooltip: currentIsCover ? 'Current cover' : 'Set as cover',
                icon: Icon(currentIsCover
                    ? Icons.wallpaper
                    : Icons.wallpaper_outlined),
                onPressed: currentIsCover
                    ? null
                    : () => _setCover(pages[_currentIndex].pageNumber),
              ),
            if (async.valueOrNull?.isOwner ?? false) ...[
              IconButton(
                tooltip: 'Transform',
                icon: const Icon(Icons.auto_awesome),
                onPressed: () => context.push('/doc/${widget.docId}/transform'),
              ),
              IconButton(
                tooltip: 'Share',
                icon: const Icon(Icons.share),
                onPressed: () => context.push('/doc/${widget.docId}/share'),
              ),
            ],
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
                      onPageChanged: (i) => setState(() => _currentIndex = i),
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
