import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Carries the email being verified from the register screen to the verify screen.
final pendingEmailProvider = StateProvider<String?>((ref) => null);
