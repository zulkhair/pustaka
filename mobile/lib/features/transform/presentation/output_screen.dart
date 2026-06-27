import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../data/models/output.dart';
import '../data/transform_repository.dart';
import 'widgets/output_view.dart';

class OutputScreen extends ConsumerWidget {
  const OutputScreen({super.key, required this.outputId});

  final String outputId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return FutureBuilder<Output>(
      future: ref.read(transformRepositoryProvider).getOutput(outputId),
      builder: (context, snap) {
        return Scaffold(
          appBar: AppBar(
            title: const Text('Output'),
            actions: [
              if (snap.hasData)
                IconButton(
                  tooltip: 'Copy',
                  icon: const Icon(Icons.copy),
                  onPressed: () {
                    Clipboard.setData(ClipboardData(text: snap.data!.content));
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(content: Text('Copied to clipboard')),
                    );
                  },
                ),
            ],
          ),
          body: switch (snap.connectionState != ConnectionState.done) {
            true => const Center(child: CircularProgressIndicator()),
            false => snap.hasError || snap.data == null
                ? Center(child: Text('Failed to load: ${snap.error}'))
                : OutputView(content: snap.data!.content),
          },
        );
      },
    );
  }
}
