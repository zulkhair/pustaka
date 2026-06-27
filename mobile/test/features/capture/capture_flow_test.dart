import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:image_picker/image_picker.dart';
import 'package:mocktail/mocktail.dart';
import 'package:pustaka/core/capture/image_capture.dart';
import 'package:pustaka/core/capture/image_compressor.dart';
import 'package:pustaka/features/capture/data/capture_repository.dart';
import 'package:pustaka/features/capture/data/models/page_upload_result.dart';
import 'package:pustaka/features/capture/presentation/capture_screen.dart';
import 'package:pustaka/features/capture/presentation/widgets/page_review_tile.dart';

class MockCaptureRepository extends Mock implements CaptureRepository {}

class MockImageCapture extends Mock implements ImageCapture {}

void main() {
  setUpAll(() => registerFallbackValue(Uint8List(0)));

  testWidgets('capture adds a reviewed page showing the OCR text',
      (tester) async {
    final repo = MockCaptureRepository();
    final cap = MockImageCapture();
    when(() => cap.takePhoto()).thenAnswer((_) async =>
        XFile.fromData(Uint8List.fromList([1, 2, 3]), name: 'p.jpg'));
    when(() => repo.uploadPage(any(), any())).thenAnswer(
        (_) async => const PageUploadResult(pageNumber: 1, ocrText: 'Hello'));

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          captureRepositoryProvider.overrideWithValue(repo),
          imageCaptureProvider.overrideWithValue(cap),
          imageCompressorProvider.overrideWithValue(PureDartImageCompressor()),
        ],
        child: const MaterialApp(home: CaptureScreen(docId: 'd1')),
      ),
    );

    await tester.tap(find.text('Capture page'));
    await tester.pumpAndSettle();
    expect(find.text('Hello'), findsOneWidget);
    expect(find.byType(PageReviewTile), findsOneWidget);

    await tester.tap(find.text('Capture page'));
    await tester.pumpAndSettle();
    expect(find.byType(PageReviewTile), findsNWidgets(2));
  });
}
