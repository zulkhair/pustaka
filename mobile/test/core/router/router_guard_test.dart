import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pustaka/core/auth/auth_controller.dart';
import 'package:pustaka/core/auth/auth_state.dart';
import 'package:pustaka/core/router/app_router.dart';

class _FakeAuth extends AuthController {
  _FakeAuth(this._status);
  final AuthStatus _status;

  @override
  AuthState build() => AuthState(status: _status);
}

Future<void> _pump(WidgetTester tester, AuthStatus status) async {
  await tester.pumpWidget(
    ProviderScope(
      overrides: [authControllerProvider.overrideWith(() => _FakeAuth(status))],
      child: Consumer(
        builder: (context, ref, _) =>
            MaterialApp.router(routerConfig: ref.watch(appRouterProvider)),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('unauthenticated redirects to /login', (tester) async {
    await _pump(tester, AuthStatus.unauthenticated);
    expect(find.text('Login'), findsWidgets);
  });

  testWidgets('unverified redirects to /verify', (tester) async {
    await _pump(tester, AuthStatus.unverified);
    expect(find.text('Verify'), findsWidgets);
  });

  testWidgets('authenticated stays on library', (tester) async {
    await _pump(tester, AuthStatus.authenticated);
    expect(find.text('Library'), findsWidgets);
  });
}
