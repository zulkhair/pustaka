import 'package:flutter/material.dart';

import '../../application/capture_controller.dart';

class PageReviewTile extends StatelessWidget {
  const PageReviewTile({
    super.key,
    required this.page,
    required this.onRetry,
    required this.onRerun,
  });

  final CapturedPage page;
  final VoidCallback onRetry;
  final VoidCallback onRerun;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Text('Page ${page.pageNumber}',
                    style: const TextStyle(fontWeight: FontWeight.bold)),
                const Spacer(),
                _statusWidget(context),
              ],
            ),
            const SizedBox(height: 8),
            if (page.status == CaptureStatus.done)
              SelectableText(page.ocrText.isEmpty ? '(no text)' : page.ocrText),
          ],
        ),
      ),
    );
  }

  Widget _statusWidget(BuildContext context) {
    switch (page.status) {
      case CaptureStatus.uploading:
        return const SizedBox(
          height: 18,
          width: 18,
          child: CircularProgressIndicator(strokeWidth: 2),
        );
      case CaptureStatus.done:
        return Row(
          children: [
            const Icon(Icons.check_circle, color: Colors.green, size: 20),
            IconButton(
              tooltip: 'Re-run OCR',
              icon: const Icon(Icons.refresh, size: 20),
              onPressed: onRerun,
            ),
          ],
        );
      case CaptureStatus.failed:
        return TextButton.icon(
          onPressed: onRetry,
          icon: const Icon(Icons.error, color: Colors.red, size: 20),
          label: const Text('Retry'),
        );
    }
  }
}
