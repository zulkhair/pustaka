import 'package:flutter_test/flutter_test.dart';

import '../../support/fake_token_storage.dart';

void main() {
  test('write then read round-trips', () async {
    final s = FakeTokenStorage();
    await s.write(access: 'a', refresh: 'r');
    final t = await s.read();
    expect(t.access, 'a');
    expect(t.refresh, 'r');
  });

  test('clear empties both', () async {
    final s = FakeTokenStorage();
    await s.write(access: 'a', refresh: 'r');
    await s.clear();
    final t = await s.read();
    expect(t.access, isNull);
    expect(t.refresh, isNull);
  });
}
