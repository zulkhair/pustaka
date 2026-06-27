import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/di/providers.dart';

/// Renders an image fetched through ApiClient.getBytes so the JWT is attached
/// (plain Image.network can't send the bearer token).
class NetworkImageAuth extends ConsumerWidget {
  const NetworkImageAuth({
    super.key,
    required this.path,
    this.width,
    this.height,
    this.fit = BoxFit.cover,
  });

  final String path;
  final double? width;
  final double? height;
  final BoxFit fit;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return FutureBuilder<Uint8List>(
      future: ref.read(apiClientProvider).getBytes(path),
      builder: (context, snap) {
        if (snap.connectionState != ConnectionState.done) {
          return SizedBox(
            width: width,
            height: height,
            child:
                const Center(child: CircularProgressIndicator(strokeWidth: 2)),
          );
        }
        if (snap.hasError || snap.data == null) {
          return SizedBox(
            width: width,
            height: height,
            child: const Icon(Icons.broken_image_outlined),
          );
        }
        return Image.memory(snap.data!, width: width, height: height, fit: fit);
      },
    );
  }
}
