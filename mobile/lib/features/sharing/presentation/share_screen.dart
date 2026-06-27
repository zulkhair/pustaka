import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/error/failure.dart';
import '../application/share_controller.dart';

class ShareScreen extends ConsumerStatefulWidget {
  const ShareScreen({super.key, required this.docId});

  final String docId;

  @override
  ConsumerState<ShareScreen> createState() => _ShareScreenState();
}

class _ShareScreenState extends ConsumerState<ShareScreen> {
  final _email = TextEditingController();
  bool _busy = false;

  @override
  void dispose() {
    _email.dispose();
    super.dispose();
  }

  Future<void> _share() async {
    final email = _email.text.trim();
    if (email.isEmpty) return;
    setState(() => _busy = true);
    try {
      await ref.read(shareControllerProvider(widget.docId).notifier).add(email);
      _email.clear();
    } on Failure catch (f) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(f.message)));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final async = ref.watch(shareControllerProvider(widget.docId));
    return Scaffold(
      appBar: AppBar(title: const Text('Share')),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(16),
            child: Row(
              children: [
                Expanded(
                  child: TextField(
                    key: const Key('shareEmailField'),
                    controller: _email,
                    decoration:
                        const InputDecoration(labelText: 'Recipient email'),
                  ),
                ),
                const SizedBox(width: 12),
                FilledButton(
                  onPressed: _busy ? null : _share,
                  child: const Text('Share'),
                ),
              ],
            ),
          ),
          Expanded(
            child: async.when(
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (e, _) => Center(child: Text('Failed to load: $e')),
              data: (shares) => shares.isEmpty
                  ? const Center(child: Text('Not shared with anyone yet'))
                  : ListView.builder(
                      itemCount: shares.length,
                      itemBuilder: (context, i) {
                        final s = shares[i];
                        return ListTile(
                          leading: const Icon(Icons.person_outline),
                          title: Text(s.email.isEmpty ? s.userId : s.email),
                          subtitle: Text(s.permission),
                          trailing: IconButton(
                            icon: const Icon(Icons.delete_outline),
                            onPressed: () => ref
                                .read(shareControllerProvider(widget.docId)
                                    .notifier)
                                .revoke(s.userId),
                          ),
                        );
                      },
                    ),
            ),
          ),
        ],
      ),
    );
  }
}
