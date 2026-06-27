import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../error/failure.dart';
import 'auth_service.dart';
import 'auth_state.dart';
import 'token_storage.dart';

/// Default token storage. Overridden in tests.
final tokenStorageProvider =
    Provider<TokenStorage>((ref) => SecureTokenStorage());

/// Auth service over the authenticated ApiClient. Wired by DI (Task 8); throws
/// if read before being overridden.
final authServiceProvider = Provider<AuthService>(
  (ref) =>
      throw UnimplementedError('authServiceProvider must be overridden by DI'),
);

final authControllerProvider =
    NotifierProvider<AuthController, AuthState>(AuthController.new);

class AuthController extends Notifier<AuthState> {
  AuthService get _svc => ref.read(authServiceProvider);
  TokenStorage get _store => ref.read(tokenStorageProvider);

  @override
  AuthState build() => const AuthState();

  AuthStatus _statusFor(bool verified) =>
      verified ? AuthStatus.authenticated : AuthStatus.unverified;

  Future<void> bootstrap() async {
    final t = await _store.read();
    if (t.access == null || t.refresh == null) {
      state = const AuthState(status: AuthStatus.unauthenticated);
      return;
    }
    try {
      final user = await _svc.me();
      state = AuthState(status: _statusFor(user.emailVerified), user: user);
    } on Failure {
      await _store.clear();
      state = const AuthState(status: AuthStatus.unauthenticated);
    }
  }

  Future<void> register({
    required String username,
    required String email,
    required String password,
  }) async {
    state = state.copyWith(busy: true, error: null);
    try {
      await _svc.register(username: username, email: email, password: password);
      state = state.copyWith(busy: false);
    } on Failure catch (f) {
      state = state.copyWith(busy: false, error: f.message);
    }
  }

  Future<void> verifyEmail(
      {required String email, required String code}) async {
    state = state.copyWith(busy: true, error: null);
    try {
      final tokens = await _svc.verifyEmail(email: email, code: code);
      await _store.write(access: tokens.access, refresh: tokens.refresh);
      final user = await _svc.me();
      state = AuthState(status: _statusFor(user.emailVerified), user: user);
    } on Failure catch (f) {
      state = state.copyWith(busy: false, error: f.message);
    }
  }

  Future<void> resend(String email) async {
    try {
      await _svc.resend(email);
    } on Failure {
      // resend is best-effort; the backend is enumeration-resistant anyway.
    }
  }

  Future<void> login(
      {required String identifier, required String password}) async {
    state = state.copyWith(busy: true, error: null);
    try {
      final tokens =
          await _svc.login(identifier: identifier, password: password);
      await _store.write(access: tokens.access, refresh: tokens.refresh);
      final user = await _svc.me();
      state = AuthState(status: _statusFor(user.emailVerified), user: user);
    } on Failure catch (f) {
      state = state.copyWith(busy: false, error: f.message);
    }
  }

  Future<void> logout() async {
    final t = await _store.read();
    final refresh = t.refresh;
    if (refresh != null) {
      try {
        await _svc.logout(refresh);
      } on Failure {
        // ignore; we clear locally regardless
      }
    }
    await _store.clear();
    state = const AuthState(status: AuthStatus.unauthenticated);
  }

  /// Called by the interceptor. Rotates tokens; returns the new access token or
  /// null (and logs out) on failure.
  Future<String?> refreshTokens() async {
    final t = await _store.read();
    final refresh = t.refresh;
    if (refresh == null) {
      markLoggedOut();
      return null;
    }
    try {
      final tokens = await _svc.refresh(refresh);
      await _store.write(access: tokens.access, refresh: tokens.refresh);
      return tokens.access;
    } on Failure {
      markLoggedOut();
      return null;
    }
  }

  /// Synchronous: clear storage (fire-and-forget) + set unauthenticated, no network.
  void markLoggedOut() {
    _store.clear();
    state = const AuthState(status: AuthStatus.unauthenticated);
  }
}
