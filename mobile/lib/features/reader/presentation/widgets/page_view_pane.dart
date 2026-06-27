import 'package:flutter/material.dart';

import '../../../../shared/widgets/network_image_auth.dart';
import '../../data/models/page.dart';
import '../../data/reader_repository.dart';

/// Shows one page: the scanned image (pinch-zoom) or its OCR text.
class PageViewPane extends StatelessWidget {
  const PageViewPane({
    super.key,
    required this.docId,
    required this.page,
    required this.showImage,
  });

  final String docId;
  final DocPage page;
  final bool showImage;

  @override
  Widget build(BuildContext context) {
    final asImage = showImage && page.hasImage;
    return InteractiveViewer(
      maxScale: 5,
      child: asImage
          ? Center(
              child:
                  NetworkImageAuth(path: pageImagePath(docId, page.pageNumber)))
          : SingleChildScrollView(
              padding: const EdgeInsets.all(16),
              child: SelectableText(
                  page.ocrText.isEmpty ? '(no text yet)' : page.ocrText),
            ),
    );
  }
}
