import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pustaka/app.dart';

void main() {
  testWidgets('app boots and shows placeholder', (tester) async {
    await tester.pumpWidget(const ProviderScope(child: PustakaApp()));
    expect(find.text('Pustaka'), findsOneWidget);
  });
}
