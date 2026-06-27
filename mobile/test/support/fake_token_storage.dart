import 'package:pustaka/core/auth/token_storage.dart';

/// In-memory TokenStorage for tests (no platform channel).
class FakeTokenStorage implements TokenStorage {
  String? _access;
  String? _refresh;

  @override
  Future<({String? access, String? refresh})> read() async =>
      (access: _access, refresh: _refresh);

  @override
  Future<void> write({required String access, required String refresh}) async {
    _access = access;
    _refresh = refresh;
  }

  @override
  Future<void> clear() async {
    _access = null;
    _refresh = null;
  }
}
