import 'package:flutter_test/flutter_test.dart';
import 'package:pustaka/core/update/version_service.dart';

import '../../support/fake_api.dart';

void main() {
  group('compareSemver', () {
    test('orders versions numerically', () {
      expect(compareSemver('1.2.0', '1.1.0'), greaterThan(0));
      expect(compareSemver('1.1.0', '1.1.0'), 0);
      expect(compareSemver('1.0.0', '1.1.0'), lessThan(0));
      expect(compareSemver('1.2', '1.2.0'), 0);
    });
  });

  group('VersionService.check', () {
    test('newer server version => updateAvailable', () async {
      final svc = VersionService(
        apiClientReturningData(
            {'version': '1.2.0', 'downloadUrl': 'http://x/apk'}),
        currentVersionOverride: '1.1.0',
      );
      final info = await svc.check();
      expect(info.updateAvailable, isTrue);
      expect(info.latest, '1.2.0');
      expect(info.url, 'http://x/apk');
    });

    test('equal version => no update', () async {
      final svc = VersionService(
        apiClientReturningData({'version': '1.1.0', 'downloadUrl': 'x'}),
        currentVersionOverride: '1.1.0',
      );
      expect((await svc.check()).updateAvailable, isFalse);
    });

    test('older server version => no update', () async {
      final svc = VersionService(
        apiClientReturningData({'version': '1.0.0', 'downloadUrl': 'x'}),
        currentVersionOverride: '1.1.0',
      );
      expect((await svc.check()).updateAvailable, isFalse);
    });

    test('mandatory flag passes through', () async {
      final svc = VersionService(
        apiClientReturningData(
            {'version': '2.0.0', 'downloadUrl': 'x', 'mandatory': true}),
        currentVersionOverride: '1.1.0',
      );
      final info = await svc.check();
      expect(info.mandatory, isTrue);
    });
  });
}
