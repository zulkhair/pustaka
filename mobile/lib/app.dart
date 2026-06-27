import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Root widget. The router + theme are wired in Tasks 7–9; this is a placeholder
/// so the scaffold compiles and the smoke test passes.
class PustakaApp extends ConsumerWidget {
  const PustakaApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return const MaterialApp(
      home: Scaffold(
        body: Center(child: Text('Pustaka')),
      ),
    );
  }
}
