import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:package_info_plus/package_info_plus.dart';

import '../api/api_client.dart';
import '../di/providers.dart';

typedef VersionInfo = ({
  bool updateAvailable,
  bool mandatory,
  String latest,
  String url
});

/// Compares dotted semver strings. >0 if a is newer than b.
int compareSemver(String a, String b) {
  final pa = a.split('.').map((s) => int.tryParse(s.trim()) ?? 0).toList();
  final pb = b.split('.').map((s) => int.tryParse(s.trim()) ?? 0).toList();
  for (var i = 0; i < 3; i++) {
    final x = i < pa.length ? pa[i] : 0;
    final y = i < pb.length ? pb[i] : 0;
    if (x != y) return x.compareTo(y);
  }
  return 0;
}

class VersionService {
  VersionService(this._client, {String? currentVersionOverride})
      : _override = currentVersionOverride;

  final ApiClient _client;
  final String? _override;

  Future<VersionInfo> check() async {
    final remote =
        await _client.get<({String version, String url, bool mandatory})>(
      '/version',
      parse: (d) {
        final m = d! as Map<String, dynamic>;
        return (
          version: m['version'] as String? ?? '0.0.0',
          url: (m['downloadUrl'] ?? m['url']) as String? ?? '',
          mandatory: m['mandatory'] as bool? ?? false,
        );
      },
    );
    final current = _override ?? (await PackageInfo.fromPlatform()).version;
    return (
      updateAvailable: compareSemver(remote.version, current) > 0,
      mandatory: remote.mandatory,
      latest: remote.version,
      url: remote.url,
    );
  }
}

final versionServiceProvider = Provider<VersionService>(
    (ref) => VersionService(ref.watch(apiClientProvider)));
