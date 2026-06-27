import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../templates/application/templates_controller.dart';
import '../application/transform_controller.dart';
import 'widgets/output_view.dart';

class TransformScreen extends ConsumerStatefulWidget {
  const TransformScreen({super.key, required this.docId});

  final String docId;

  @override
  ConsumerState<TransformScreen> createState() => _TransformScreenState();
}

class _TransformScreenState extends ConsumerState<TransformScreen> {
  String? _selected;

  @override
  Widget build(BuildContext context) {
    final templates = ref.watch(templatesControllerProvider);
    final state = ref.watch(transformControllerProvider(widget.docId));
    return Scaffold(
      appBar: AppBar(title: const Text('Transform')),
      body: templates.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) => Center(child: Text('Failed to load templates: $e')),
        data: (list) => Column(
          children: [
            Expanded(
              child: ListView(
                children: [
                  for (final t in list)
                    RadioListTile<String>(
                      value: t.id,
                      groupValue: _selected,
                      onChanged: (v) => setState(() => _selected = v),
                      title: Text(t.name),
                      subtitle:
                          Text('${t.scope.name} · ${t.outputFormat.name}'),
                    ),
                  if (state.status == TransformStatus.running)
                    const Padding(
                      padding: EdgeInsets.all(16),
                      child: Column(
                        children: [
                          CircularProgressIndicator(),
                          SizedBox(height: 8),
                          Text('Running on the server…'),
                        ],
                      ),
                    ),
                  if (state.status == TransformStatus.failed)
                    Padding(
                      padding: const EdgeInsets.all(16),
                      child: Text(state.error ?? 'Failed',
                          style: const TextStyle(color: Colors.red)),
                    ),
                  if (state.status == TransformStatus.done &&
                      state.output != null)
                    OutputView(content: state.output!.content),
                ],
              ),
            ),
            SafeArea(
              child: Padding(
                padding: const EdgeInsets.all(12),
                child: FilledButton.icon(
                  onPressed: (_selected == null ||
                          state.status == TransformStatus.running)
                      ? null
                      : () => ref
                          .read(transformControllerProvider(widget.docId)
                              .notifier)
                          .run(_selected!),
                  icon: const Icon(Icons.play_arrow),
                  label: const Text('Run'),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
