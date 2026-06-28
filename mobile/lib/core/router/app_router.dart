import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../features/auth/presentation/login_screen.dart';
import '../../features/auth/presentation/register_screen.dart';
import '../../features/auth/presentation/verify_email_screen.dart';
import '../../features/capture/presentation/capture_screen.dart';
import '../../features/library/presentation/library_screen.dart';
import '../../features/reader/presentation/reader_screen.dart';
import '../../features/sharing/presentation/share_screen.dart';
import '../../features/templates/presentation/templates_screen.dart';
import '../../features/transform/presentation/output_screen.dart';
import '../../features/transform/presentation/transform_screen.dart';
import '../auth/auth_controller.dart';
import '../auth/auth_state.dart';
import '../update/update_gate.dart';

/// Wraps every authenticated route. The single mount point for [UpdateGate] —
/// the OTA prompt runs once, only when authenticated.
class AppShell extends StatelessWidget {
  const AppShell({super.key, required this.child});
  final Widget child;

  @override
  Widget build(BuildContext context) => UpdateGate(child: child);
}

final appRouterProvider = Provider<GoRouter>((ref) {
  // Bridge the auth status into a Listenable so GoRouter re-evaluates redirect.
  final notifier =
      ValueNotifier<AuthStatus>(ref.read(authControllerProvider).status);
  ref.onDispose(notifier.dispose);
  ref.listen<AuthState>(
      authControllerProvider, (_, next) => notifier.value = next.status);

  return GoRouter(
    initialLocation: '/',
    refreshListenable: notifier,
    redirect: (context, state) {
      final status = ref.read(authControllerProvider).status;
      final loc = state.matchedLocation;
      final onAuthRoute =
          loc == '/login' || loc == '/register' || loc == '/verify';
      switch (status) {
        case AuthStatus.unknown:
          return null; // app.dart shows a splash while unknown
        case AuthStatus.unauthenticated:
          return onAuthRoute ? null : '/login';
        case AuthStatus.unverified:
          return loc == '/verify' ? null : '/verify';
        case AuthStatus.authenticated:
          return onAuthRoute ? '/' : null;
      }
    },
    routes: [
      GoRoute(
          name: 'login',
          path: '/login',
          builder: (c, s) => const LoginScreen()),
      GoRoute(
          name: 'register',
          path: '/register',
          builder: (c, s) => const RegisterScreen()),
      GoRoute(
          name: 'verify',
          path: '/verify',
          builder: (c, s) => const VerifyEmailScreen()),
      ShellRoute(
        builder: (c, s, child) => AppShell(child: child),
        routes: [
          GoRoute(
              name: 'library',
              path: '/',
              builder: (c, s) => const LibraryScreen()),
          GoRoute(
              name: 'capture',
              path: '/capture',
              builder: (c, s) => const CaptureFlowScreen()),
          GoRoute(
              name: 'reader',
              path: '/doc/:id',
              builder: (c, s) => ReaderScreen(docId: s.pathParameters['id']!)),
          GoRoute(
              name: 'resume',
              path: '/doc/:id/capture',
              builder: (c, s) => CaptureScreen(docId: s.pathParameters['id']!)),
          GoRoute(
            name: 'transform',
            path: '/doc/:id/transform',
            builder: (c, s) => TransformScreen(docId: s.pathParameters['id']!),
          ),
          GoRoute(
              name: 'output',
              path: '/output/:id',
              builder: (c, s) =>
                  OutputScreen(outputId: s.pathParameters['id']!)),
          GoRoute(
              name: 'templates',
              path: '/templates',
              builder: (c, s) => const TemplatesScreen()),
          GoRoute(
              name: 'share',
              path: '/doc/:id/share',
              builder: (c, s) => ShareScreen(docId: s.pathParameters['id']!)),
        ],
      ),
    ],
  );
});
