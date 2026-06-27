import '../api/api_client.dart';
import 'app_user.dart';
import 'tokens.dart';

/// Thin wrapper over [ApiClient] — one method per auth endpoint.
class AuthService {
  AuthService(this._client);
  final ApiClient _client;

  Future<void> register({
    required String username,
    required String email,
    required String password,
  }) async {
    await _client.post<Object?>(
      '/auth/register',
      body: {'username': username, 'email': email, 'password': password},
      parse: (_) => null,
    );
  }

  Future<Tokens> verifyEmail({required String email, required String code}) {
    return _client.post<Tokens>(
      '/auth/verify-email',
      body: {'email': email, 'code': code},
      parse: (d) => Tokens.fromJson(d! as Map<String, dynamic>),
    );
  }

  Future<void> resend(String email) async {
    await _client.post<Object?>(
      '/auth/resend-verification',
      body: {'email': email},
      parse: (_) => null,
    );
  }

  Future<Tokens> login({required String identifier, required String password}) {
    return _client.post<Tokens>(
      '/auth/login',
      body: {'identifier': identifier, 'password': password},
      parse: (d) => Tokens.fromJson(d! as Map<String, dynamic>),
    );
  }

  Future<Tokens> refresh(String refreshToken) {
    return _client.post<Tokens>(
      '/auth/refresh',
      body: {'refreshToken': refreshToken},
      parse: (d) => Tokens.fromJson(d! as Map<String, dynamic>),
    );
  }

  Future<void> logout(String refreshToken) async {
    await _client.post<Object?>(
      '/auth/logout',
      body: {'refreshToken': refreshToken},
      parse: (_) => null,
    );
  }

  Future<AppUser> me() {
    return _client.get<AppUser>(
      '/auth/me',
      parse: (d) => AppUser.fromJson(d! as Map<String, dynamic>),
    );
  }
}
