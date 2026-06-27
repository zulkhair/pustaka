import 'package:flutter_test/flutter_test.dart';
import 'package:pustaka/core/auth/app_user.dart';
import 'package:pustaka/core/auth/tokens.dart';

void main() {
  test('AppUser.fromJson maps fields', () {
    final u = AppUser.fromJson({
      'id': 'u1',
      'username': 'a',
      'email': 'a@b.c',
      'role': 'user',
      'emailVerified': false,
    });
    expect(u.id, 'u1');
    expect(u.username, 'a');
    expect(u.email, 'a@b.c');
    expect(u.role, 'user');
    expect(u.emailVerified, isFalse);
  });

  test('Tokens.fromJson maps camelCase fields', () {
    final t = Tokens.fromJson(
        {'accessToken': 'x', 'refreshToken': 'y', 'expiresIn': 900});
    expect(t.access, 'x');
    expect(t.refresh, 'y');
    expect(t.expiresIn, 900);
  });
}
