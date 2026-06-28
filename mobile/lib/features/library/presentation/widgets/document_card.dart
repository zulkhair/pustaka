import 'package:flutter/material.dart';

import '../../../../shared/widgets/network_image_auth.dart';
import '../../data/models/document.dart';

class DocumentCard extends StatelessWidget {
  const DocumentCard({
    super.key,
    required this.doc,
    required this.onTap,
    this.onResume,
    this.onRename,
    this.onDelete,
  });

  final Document doc;
  final VoidCallback onTap;

  /// Owner-only actions. When all null, no menu is shown.
  final VoidCallback? onResume;
  final VoidCallback? onRename;
  final VoidCallback? onDelete;

  bool get _hasMenu =>
      onResume != null || onRename != null || onDelete != null;

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
              child: Stack(
                fit: StackFit.expand,
                children: [
                  doc.thumbUrl != null
                      ? NetworkImageAuth(path: doc.thumbUrl!)
                      : Container(
                          color: Colors.black12,
                          child:
                              const Icon(Icons.description_outlined, size: 48),
                        ),
                  if (_hasMenu)
                    Positioned(
                      top: 0,
                      right: 0,
                      child: _Menu(
                        onResume: onResume,
                        onRename: onRename,
                        onDelete: onDelete,
                      ),
                    ),
                ],
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

class _Menu extends StatelessWidget {
  const _Menu({this.onResume, this.onRename, this.onDelete});

  final VoidCallback? onResume;
  final VoidCallback? onRename;
  final VoidCallback? onDelete;

  @override
  Widget build(BuildContext context) {
    return PopupMenuButton<String>(
      icon: const Icon(Icons.more_vert),
      tooltip: 'Actions',
      onSelected: (v) {
        switch (v) {
          case 'resume':
            onResume?.call();
          case 'rename':
            onRename?.call();
          case 'delete':
            onDelete?.call();
        }
      },
      itemBuilder: (_) => [
        if (onResume != null)
          const PopupMenuItem(
            value: 'resume',
            child: ListTile(
              leading: Icon(Icons.add_a_photo_outlined),
              title: Text('Resume scanning'),
            ),
          ),
        if (onRename != null)
          const PopupMenuItem(
            value: 'rename',
            child: ListTile(
              leading: Icon(Icons.edit_outlined),
              title: Text('Rename'),
            ),
          ),
        if (onDelete != null)
          const PopupMenuItem(
            value: 'delete',
            child: ListTile(
              leading: Icon(Icons.delete_outline),
              title: Text('Delete'),
            ),
          ),
      ],
    );
  }
}
