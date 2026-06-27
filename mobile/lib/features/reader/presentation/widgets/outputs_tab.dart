import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../../transform/data/models/output.dart';

class OutputsTab extends StatelessWidget {
  const OutputsTab({super.key, required this.outputs});

  final List<Output> outputs;

  @override
  Widget build(BuildContext context) {
    if (outputs.isEmpty) {
      return const Center(child: Text('No outputs yet'));
    }
    return ListView.builder(
      itemCount: outputs.length,
      itemBuilder: (context, i) {
        final o = outputs[i];
        return ListTile(
          leading: const Icon(Icons.description_outlined),
          title: Text('Output ${o.id.substring(0, o.id.length.clamp(0, 8))}'),
          subtitle: Text('${o.status.name} · ${o.model}'),
          onTap: () => context.go('/output/${o.id}'),
        );
      },
    );
  }
}
