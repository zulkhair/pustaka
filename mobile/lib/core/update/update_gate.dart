import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import 'version_service.dart';

/// Wraps the authenticated app; on first build it checks for a newer version
/// and prompts an update. Mounted once inside AppShell.
class UpdateGate extends ConsumerStatefulWidget {
  const UpdateGate({super.key, required this.child});

  final Widget child;

  @override
  ConsumerState<UpdateGate> createState() => _UpdateGateState();
}

class _UpdateGateState extends ConsumerState<UpdateGate> {
  bool _checked = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _check());
  }

  Future<void> _check() async {
    if (_checked) return;
    _checked = true;
    try {
      final info = await ref.read(versionServiceProvider).check();
      if (info.updateAvailable && mounted) {
        _prompt(info);
      }
    } catch (_) {
      // OTA check is best-effort; ignore failures.
    }
  }

  void _prompt(VersionInfo info) {
    showDialog<void>(
      context: context,
      barrierDismissible: !info.mandatory,
      builder: (context) => AlertDialog(
        title: const Text('Update available'),
        content: Text('Version ${info.latest} is available.'),
        actions: [
          if (!info.mandatory)
            TextButton(
                onPressed: () => Navigator.of(context).pop(),
                child: const Text('Later')),
          FilledButton(
            onPressed: () {
              if (info.url.isNotEmpty) {
                launchUrl(Uri.parse(info.url),
                    mode: LaunchMode.externalApplication);
              }
            },
            child: const Text('Update'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) => widget.child;
}
