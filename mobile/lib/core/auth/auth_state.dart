import 'app_user.dart';

enum AuthStatus { unknown, unauthenticated, unverified, authenticated }

const Object _noChange = Object();

class AuthState {
  const AuthState({
    this.status = AuthStatus.unknown,
    this.user,
    this.busy = false,
    this.error,
  });

  final AuthStatus status;
  final AppUser? user;
  final bool busy;
  final String? error;

  AuthState copyWith({
    AuthStatus? status,
    AppUser? user,
    bool? busy,
    Object? error = _noChange,
  }) {
    return AuthState(
      status: status ?? this.status,
      user: user ?? this.user,
      busy: busy ?? this.busy,
      error: identical(error, _noChange) ? this.error : error as String?,
    );
  }
}
