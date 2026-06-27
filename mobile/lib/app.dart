import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'core/auth/auth_controller.dart';
import 'core/auth/auth_state.dart';
import 'core/router/app_router.dart';
import 'core/theme/app_theme.dart';

class PustakaApp extends ConsumerStatefulWidget {
  const PustakaApp({super.key});

  @override
  ConsumerState<PustakaApp> createState() => _PustakaAppState();
}

class _PustakaAppState extends ConsumerState<PustakaApp> {
  @override
  void initState() {
    super.initState();
    // Read stored tokens → resolve auth status once at launch.
    Future.microtask(
        () => ref.read(authControllerProvider.notifier).bootstrap());
  }

  @override
  Widget build(BuildContext context) {
    final status = ref.watch(authControllerProvider.select((s) => s.status));
    if (status == AuthStatus.unknown) {
      return MaterialApp(
        theme: appTheme,
        debugShowCheckedModeBanner: false,
        home: const Scaffold(body: Center(child: CircularProgressIndicator())),
      );
    }
    return MaterialApp.router(
      title: 'Pustaka',
      theme: appTheme,
      debugShowCheckedModeBanner: false,
      routerConfig: ref.watch(appRouterProvider),
    );
  }
}
