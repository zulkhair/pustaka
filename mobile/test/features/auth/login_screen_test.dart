import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pustaka/core/auth/auth_controller.dart';
import 'package:pustaka/core/auth/auth_state.dart';
import 'package:pustaka/features/auth/presentation/login_screen.dart';

class _RecAuth extends AuthController {
  _RecAuth(this._initial);
  final AuthState _initial;
  int calls = 0;
  String? id;
  String? pw;

  @override
  AuthState build() => _initial;

  @override
  Future<void> login(
      {required String identifier, required String password}) async {
    calls++;
    id = identifier;
    pw = password;
  }
}

Future<void> _pump(WidgetTester tester, _RecAuth fake) async {
  await tester.pumpWidget(
    ProviderScope(
      overrides: [authControllerProvider.overrideWith(() => fake)],
      child: const MaterialApp(home: LoginScreen()),
    ),
  );
}

void main() {
  testWidgets('tapping log in calls login with entered creds', (tester) async {
    final fake = _RecAuth(const AuthState(status: AuthStatus.unauthenticated));
    await _pump(tester, fake);
    await tester.enterText(
        find.byKey(const Key('identifierField')), 'me@e.com');
    await tester.enterText(find.byKey(const Key('passwordField')), 'pw');
    await tester.tap(find.byKey(const Key('loginButton')));
    await tester.pump();
    expect(fake.calls, 1);
    expect(fake.id, 'me@e.com');
    expect(fake.pw, 'pw');
  });

  testWidgets('error message is shown', (tester) async {
    final fake = _RecAuth(const AuthState(
        status: AuthStatus.unauthenticated, error: 'bad creds'));
    await _pump(tester, fake);
    expect(find.text('bad creds'), findsOneWidget);
  });

  testWidgets('busy shows a spinner and disables the button', (tester) async {
    final fake = _RecAuth(
        const AuthState(status: AuthStatus.unauthenticated, busy: true));
    await _pump(tester, fake);
    expect(find.byType(CircularProgressIndicator), findsOneWidget);
    await tester.tap(find.byKey(const Key('loginButton')));
    await tester.pump();
    expect(fake.calls, 0);
  });
}
