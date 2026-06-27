import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../core/auth/auth_controller.dart';

class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});

  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen> {
  final _identifier = TextEditingController();
  final _password = TextEditingController();

  @override
  void dispose() {
    _identifier.dispose();
    _password.dispose();
    super.dispose();
  }

  void _submit() {
    ref.read(authControllerProvider.notifier).login(
          identifier: _identifier.text.trim(),
          password: _password.text,
        );
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(authControllerProvider);
    return Scaffold(
      appBar: AppBar(title: const Text('Log in')),
      body: Padding(
        padding: const EdgeInsets.all(24),
        child: ListView(
          children: [
            TextField(
              key: const Key('identifierField'),
              controller: _identifier,
              decoration: const InputDecoration(labelText: 'Email or username'),
              autofillHints: const [AutofillHints.username],
            ),
            const SizedBox(height: 12),
            TextField(
              key: const Key('passwordField'),
              controller: _password,
              obscureText: true,
              decoration: const InputDecoration(labelText: 'Password'),
            ),
            if (state.error != null) ...[
              const SizedBox(height: 12),
              Text(state.error!, style: const TextStyle(color: Colors.red)),
            ],
            const SizedBox(height: 24),
            FilledButton(
              key: const Key('loginButton'),
              onPressed: state.busy ? null : _submit,
              child: state.busy
                  ? const SizedBox(
                      height: 20,
                      width: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Text('Log in'),
            ),
            TextButton(
              onPressed: () => context.push('/register'),
              child: const Text('Create account'),
            ),
          ],
        ),
      ),
    );
  }
}
