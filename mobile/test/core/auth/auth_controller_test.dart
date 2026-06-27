import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:pustaka/core/auth/app_user.dart';
import 'package:pustaka/core/auth/auth_controller.dart';
import 'package:pustaka/core/auth/auth_service.dart';
import 'package:pustaka/core/auth/auth_state.dart';
import 'package:pustaka/core/auth/tokens.dart';
import 'package:pustaka/core/error/failure.dart';

import '../../support/fake_token_storage.dart';

class MockAuthService extends Mock implements AuthService {}

AppUser _user({bool verified = true}) => AppUser(
      id: 'u1',
      username: 'a',
      email: 'a@b.c',
      role: 'user',
      emailVerified: verified,
    );

const _tokens = Tokens(access: 'na', refresh: 'nr', expiresIn: 900);

ProviderContainer _container(MockAuthService mock, FakeTokenStorage store) {
  final c = ProviderContainer(overrides: [
    authServiceProvider.overrideWithValue(mock),
    tokenStorageProvider.overrideWithValue(store),
  ]);
  addTearDown(c.dispose);
  return c;
}

void main() {
  test('bootstrap with no tokens => unauthenticated', () async {
    final mock = MockAuthService();
    final c = _container(mock, FakeTokenStorage());
    await c.read(authControllerProvider.notifier).bootstrap();
    expect(c.read(authControllerProvider).status, AuthStatus.unauthenticated);
  });

  test('bootstrap with tokens + verified me => authenticated', () async {
    final mock = MockAuthService();
    when(() => mock.me()).thenAnswer((_) async => _user(verified: true));
    final store = FakeTokenStorage()..write(access: 'a', refresh: 'r');
    final c = _container(mock, store);
    await c.read(authControllerProvider.notifier).bootstrap();
    expect(c.read(authControllerProvider).status, AuthStatus.authenticated);
  });

  test('bootstrap with unverified me => unverified', () async {
    final mock = MockAuthService();
    when(() => mock.me()).thenAnswer((_) async => _user(verified: false));
    final store = FakeTokenStorage()..write(access: 'a', refresh: 'r');
    final c = _container(mock, store);
    await c.read(authControllerProvider.notifier).bootstrap();
    expect(c.read(authControllerProvider).status, AuthStatus.unverified);
  });

  test('login success stores tokens and authenticates', () async {
    final mock = MockAuthService();
    when(() => mock.login(
        identifier: any(named: 'identifier'),
        password: any(named: 'password'))).thenAnswer((_) async => _tokens);
    when(() => mock.me()).thenAnswer((_) async => _user(verified: true));
    final store = FakeTokenStorage();
    final c = _container(mock, store);
    await c
        .read(authControllerProvider.notifier)
        .login(identifier: 'a@b.c', password: 'pw');
    expect(c.read(authControllerProvider).status, AuthStatus.authenticated);
    expect((await store.read()).access, 'na');
  });

  test('login failure sets error, not busy, status unchanged', () async {
    final mock = MockAuthService();
    when(() => mock.login(
            identifier: any(named: 'identifier'),
            password: any(named: 'password')))
        .thenThrow(const ApiFailure(1, 'bad creds'));
    final c = _container(mock, FakeTokenStorage());
    await c
        .read(authControllerProvider.notifier)
        .login(identifier: 'a', password: 'b');
    final s = c.read(authControllerProvider);
    expect(s.error, 'bad creds');
    expect(s.busy, isFalse);
    expect(s.status, AuthStatus.unknown);
  });

  test('verifyEmail success authenticates', () async {
    final mock = MockAuthService();
    when(() => mock.verifyEmail(
        email: any(named: 'email'),
        code: any(named: 'code'))).thenAnswer((_) async => _tokens);
    when(() => mock.me()).thenAnswer((_) async => _user(verified: true));
    final c = _container(mock, FakeTokenStorage());
    await c
        .read(authControllerProvider.notifier)
        .verifyEmail(email: 'a@b.c', code: '123456');
    expect(c.read(authControllerProvider).status, AuthStatus.authenticated);
  });

  test('logout clears storage and unauthenticates', () async {
    final mock = MockAuthService();
    when(() => mock.logout(any())).thenAnswer((_) async {});
    final store = FakeTokenStorage()..write(access: 'a', refresh: 'r');
    final c = _container(mock, store);
    await c.read(authControllerProvider.notifier).logout();
    expect(c.read(authControllerProvider).status, AuthStatus.unauthenticated);
    expect((await store.read()).refresh, isNull);
    verify(() => mock.logout('r')).called(1);
  });

  test('refreshTokens success rotates and returns new access', () async {
    final mock = MockAuthService();
    when(() => mock.refresh(any())).thenAnswer((_) async =>
        const Tokens(access: 'na2', refresh: 'nr2', expiresIn: 900));
    final store = FakeTokenStorage()..write(access: 'a', refresh: 'r');
    final c = _container(mock, store);
    final out = await c.read(authControllerProvider.notifier).refreshTokens();
    expect(out, 'na2');
    expect((await store.read()).access, 'na2');
  });

  test('refreshTokens failure clears and returns null', () async {
    final mock = MockAuthService();
    when(() => mock.refresh(any())).thenThrow(const ApiFailure(1, 'expired'));
    final store = FakeTokenStorage()..write(access: 'a', refresh: 'r');
    final c = _container(mock, store);
    final out = await c.read(authControllerProvider.notifier).refreshTokens();
    expect(out, isNull);
    expect(c.read(authControllerProvider).status, AuthStatus.unauthenticated);
  });

  test('markLoggedOut is synchronous with no service call', () async {
    final mock = MockAuthService();
    final store = FakeTokenStorage()..write(access: 'a', refresh: 'r');
    final c = _container(mock, store);
    c.read(authControllerProvider.notifier).markLoggedOut();
    expect(c.read(authControllerProvider).status, AuthStatus.unauthenticated);
    verifyZeroInteractions(mock);
  });
}
