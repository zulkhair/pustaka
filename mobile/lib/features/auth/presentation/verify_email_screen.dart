import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/auth/auth_controller.dart';
import '../application/pending_email.dart';
import 'widgets/code_field.dart';

class VerifyEmailScreen extends ConsumerStatefulWidget {
  const VerifyEmailScreen({super.key});

  @override
  ConsumerState<VerifyEmailScreen> createState() => _VerifyEmailScreenState();
}

class _VerifyEmailScreenState extends ConsumerState<VerifyEmailScreen> {
  final _code = TextEditingController();
  Timer? _timer;
  int _cooldown = 0;

  @override
  void dispose() {
    _timer?.cancel();
    _code.dispose();
    super.dispose();
  }

  void _startCooldown() {
    setState(() => _cooldown = 30);
    _timer?.cancel();
    _timer = Timer.periodic(const Duration(seconds: 1), (t) {
      if (_cooldown <= 1) {
        t.cancel();
        setState(() => _cooldown = 0);
      } else {
        setState(() => _cooldown--);
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(authControllerProvider);
    final email = ref.watch(pendingEmailProvider) ?? '';
    return Scaffold(
      appBar: AppBar(title: const Text('Verify email')),
      body: Padding(
        padding: const EdgeInsets.all(24),
        child: ListView(
          children: [
            Text('Enter the 6-digit code sent to $email'),
            const SizedBox(height: 16),
            CodeField(controller: _code),
            if (state.error != null) ...[
              const SizedBox(height: 12),
              Text(state.error!, style: const TextStyle(color: Colors.red)),
            ],
            const SizedBox(height: 16),
            FilledButton(
              onPressed: state.busy
                  ? null
                  : () => ref
                      .read(authControllerProvider.notifier)
                      .verifyEmail(email: email, code: _code.text.trim()),
              child: state.busy
                  ? const SizedBox(
                      height: 20,
                      width: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Text('Verify'),
            ),
            TextButton(
              onPressed: _cooldown > 0
                  ? null
                  : () {
                      ref.read(authControllerProvider.notifier).resend(email);
                      _startCooldown();
                    },
              child: Text(
                  _cooldown > 0 ? 'Resend in $_cooldown s' : 'Resend code'),
            ),
          ],
        ),
      ),
    );
  }
}
