import 'dart:convert';

import 'package:flutter/material.dart';

/// Renders output content as selectable text. JSON (object/array) is
/// pretty-printed in monospace; everything else (markdown/text/csv) is shown
/// as-is. No markdown rendering dependency for v1.
class OutputView extends StatelessWidget {
  const OutputView({super.key, required this.content});

  final String content;

  String? _tryPrettyJson(String s) {
    final t = s.trim();
    if (!(t.startsWith('{') || t.startsWith('['))) return null;
    try {
      return const JsonEncoder.withIndent('  ').convert(jsonDecode(t));
    } catch (_) {
      return null;
    }
  }

  @override
  Widget build(BuildContext context) {
    final pretty = _tryPrettyJson(content);
    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: SelectableText(
        pretty ?? content,
        style: pretty != null ? const TextStyle(fontFamily: 'monospace') : null,
      ),
    );
  }
}
