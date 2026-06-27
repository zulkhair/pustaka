import 'package:flutter/material.dart';

import '../../../../shared/widgets/network_image_auth.dart';
import '../../data/models/document.dart';

class DocumentCard extends StatelessWidget {
  const DocumentCard({super.key, required this.doc, required this.onTap});

  final Document doc;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Card(
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Expanded(
              child: doc.thumbUrl != null
                  ? NetworkImageAuth(path: doc.thumbUrl!)
                  : Container(
                      color: Colors.black12,
                      child: const Icon(Icons.description_outlined, size: 48),
                    ),
            ),
            Padding(
              padding: const EdgeInsets.all(8),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    doc.title,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontWeight: FontWeight.w600),
                  ),
                  const SizedBox(height: 4),
                  Row(
                    children: [
                      Text(doc.mode.name,
                          style: Theme.of(context).textTheme.bodySmall),
                      const Spacer(),
                      Text('${doc.pageCount}p',
                          style: Theme.of(context).textTheme.bodySmall),
                    ],
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
