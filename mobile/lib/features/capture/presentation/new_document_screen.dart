import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../library/application/library_controller.dart';
import '../../library/data/models/document.dart';

class NewDocumentScreen extends ConsumerStatefulWidget {
  const NewDocumentScreen({super.key, required this.onCreated});

  final void Function(Document doc) onCreated;

  @override
  ConsumerState<NewDocumentScreen> createState() => _NewDocumentScreenState();
}

class _NewDocumentScreenState extends ConsumerState<NewDocumentScreen> {
  final _title = TextEditingController();
  CaptureMode _mode = CaptureMode.photo;
  bool _busy = false;

  @override
  void dispose() {
    _title.dispose();
    super.dispose();
  }

  Future<void> _start() async {
    setState(() => _busy = true);
    try {
      final doc = await ref
          .read(libraryControllerProvider.notifier)
          .createDocument(
              _title.text.trim().isEmpty ? 'Untitled' : _title.text.trim(),
              _mode);
      widget.onCreated(doc);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          TextField(
            controller: _title,
            decoration: const InputDecoration(labelText: 'Title'),
          ),
          const SizedBox(height: 16),
          SegmentedButton<CaptureMode>(
            segments: const [
              ButtonSegment(
                  value: CaptureMode.photo,
                  label: Text('Photo'),
                  icon: Icon(Icons.photo)),
              ButtonSegment(
                  value: CaptureMode.text,
                  label: Text('Text'),
                  icon: Icon(Icons.text_snippet)),
            ],
            selected: {_mode},
            onSelectionChanged: (s) => setState(() => _mode = s.first),
          ),
          const SizedBox(height: 24),
          FilledButton(
            onPressed: _busy ? null : _start,
            child: _busy
                ? const SizedBox(
                    height: 20,
                    width: 20,
                    child: CircularProgressIndicator(strokeWidth: 2))
                : const Text('Start capturing'),
          ),
        ],
      ),
    );
  }
}
