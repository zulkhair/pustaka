import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../library/application/library_controller.dart';
import '../application/capture_controller.dart';
import 'new_document_screen.dart';
import 'widgets/page_review_tile.dart';

/// The `/capture` route: first the new-document form, then the capture+review UI.
class CaptureFlowScreen extends ConsumerStatefulWidget {
  const CaptureFlowScreen({super.key});

  @override
  ConsumerState<CaptureFlowScreen> createState() => _CaptureFlowScreenState();
}

class _CaptureFlowScreenState extends ConsumerState<CaptureFlowScreen> {
  String? _docId;

  @override
  Widget build(BuildContext context) {
    if (_docId == null) {
      return Scaffold(
        appBar: AppBar(title: const Text('New document')),
        body: NewDocumentScreen(
            onCreated: (doc) => setState(() => _docId = doc.id)),
      );
    }
    return CaptureScreen(docId: _docId!);
  }
}

class CaptureScreen extends ConsumerWidget {
  const CaptureScreen({super.key, required this.docId});

  final String docId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(captureControllerProvider(docId));
    final notifier = ref.read(captureControllerProvider(docId).notifier);
    return Scaffold(
      appBar: AppBar(title: const Text('Capture')),
      body: state.pages.isEmpty
          ? const Center(child: Text('Capture your first page'))
          : ListView.builder(
              padding: const EdgeInsets.all(12),
              itemCount: state.pages.length,
              itemBuilder: (context, i) => PageReviewTile(
                page: state.pages[i],
                onRetry: () => notifier.retry(i),
                onRerun: () => notifier.rerunOcr(state.pages[i].pageNumber),
              ),
            ),
      bottomNavigationBar: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Row(
            children: [
              Expanded(
                child: FilledButton.icon(
                  onPressed: state.capturing ? null : notifier.capturePage,
                  icon: const Icon(Icons.camera_alt),
                  label: const Text('Capture page'),
                ),
              ),
              const SizedBox(width: 12),
              OutlinedButton(
                onPressed: () async {
                  await ref.read(libraryControllerProvider.notifier).refresh();
                  if (context.mounted) context.go('/');
                },
                child: const Text('Finish'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
