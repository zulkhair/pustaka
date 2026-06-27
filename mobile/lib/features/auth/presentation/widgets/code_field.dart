import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

/// A single 6-digit verification code entry.
class CodeField extends StatelessWidget {
  const CodeField({super.key, required this.controller});

  final TextEditingController controller;

  @override
  Widget build(BuildContext context) {
    return TextField(
      key: const Key('codeField'),
      controller: controller,
      keyboardType: TextInputType.number,
      maxLength: 6,
      inputFormatters: [FilteringTextInputFormatter.digitsOnly],
      textAlign: TextAlign.center,
      style: const TextStyle(fontSize: 28, letterSpacing: 12),
      decoration: const InputDecoration(counterText: '', hintText: '••••••'),
    );
  }
}
