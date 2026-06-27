import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'app.dart';
import 'core/di/providers.dart';

void main() {
  runApp(ProviderScope(overrides: rootOverrides, child: const PustakaApp()));
}
