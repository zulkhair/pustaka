import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Persists the access + refresh token pair. The only place tokens live.
abstract class TokenStorage {
  Future<({String? access, String? refresh})> read();
  Future<void> write({required String access, required String refresh});
  Future<void> clear();
}

class SecureTokenStorage implements TokenStorage {
  SecureTokenStorage([FlutterSecureStorage? storage])
      : _s = storage ?? const FlutterSecureStorage();

  final FlutterSecureStorage _s;
  static const _kAccess = 'pustaka_access';
  static const _kRefresh = 'pustaka_refresh';

  @override
  Future<({String? access, String? refresh})> read() async {
    final access = await _s.read(key: _kAccess);
    final refresh = await _s.read(key: _kRefresh);
    return (access: access, refresh: refresh);
  }

  @override
  Future<void> write({required String access, required String refresh}) async {
    await _s.write(key: _kAccess, value: access);
    await _s.write(key: _kRefresh, value: refresh);
  }

  @override
  Future<void> clear() async {
    await _s.delete(key: _kAccess);
    await _s.delete(key: _kRefresh);
  }
}
