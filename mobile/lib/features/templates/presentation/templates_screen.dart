import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../application/templates_controller.dart';

class TemplatesScreen extends ConsumerWidget {
  const TemplatesScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final async = ref.watch(templatesControllerProvider);
    return Scaffold(
      appBar: AppBar(title: const Text('Templates')),
      body: async.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) => Center(child: Text('Failed to load: $e')),
        data: (templates) => ListView.builder(
          itemCount: templates.length,
          itemBuilder: (context, i) {
            final t = templates[i];
            return ListTile(
              title: Text(t.name),
              subtitle: Wrap(
                spacing: 8,
                children: [
                  Chip(label: Text(t.scope.name)),
                  Chip(label: Text(t.outputFormat.name)),
                ],
              ),
            );
          },
        ),
      ),
    );
  }
}
