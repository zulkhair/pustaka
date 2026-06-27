import 'dart:typed_data';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:image_picker/image_picker.dart';
import 'package:mocktail/mocktail.dart';
import 'package:pustaka/core/capture/image_capture.dart';
import 'package:pustaka/core/capture/image_compressor.dart';
import 'package:pustaka/features/capture/application/capture_controller.dart';
import 'package:pustaka/features/capture/data/capture_repository.dart';
import 'package:pustaka/features/capture/data/models/page_upload_result.dart';

class MockCaptureRepository extends Mock implements CaptureRepository {}

class MockImageCapture extends Mock implements ImageCapture {}

ProviderContainer _container(MockCaptureRepository repo, MockImageCapture cap) {
  final c = ProviderContainer(overrides: [
    captureRepositoryProvider.overrideWithValue(repo),
    imageCaptureProvider.overrideWithValue(cap),
    imageCompressorProvider.overrideWithValue(PureDartImageCompressor()),
  ]);
  addTearDown(c.dispose);
  return c;
}

void main() {
  setUpAll(() => registerFallbackValue(Uint8List(0)));

  test('capturePage compresses, uploads, and appends a done page', () async {
    final repo = MockCaptureRepository();
    final cap = MockImageCapture();
    when(() => cap.takePhoto()).thenAnswer((_) async =>
        XFile.fromData(Uint8List.fromList([1, 2, 3]), name: 'p.jpg'));
    when(() => repo.uploadPage(any(), any())).thenAnswer(
        (_) async => const PageUploadResult(pageNumber: 1, ocrText: 'Hello'));

    final c = _container(repo, cap);
    await c.read(captureControllerProvider('d1').notifier).capturePage();

    final pages = c.read(captureControllerProvider('d1')).pages;
    expect(pages, hasLength(1));
    expect(pages.first.ocrText, 'Hello');
    expect(pages.first.status, CaptureStatus.done);
  });

  test('upload failure marks page failed; retry succeeds', () async {
    final repo = MockCaptureRepository();
    final cap = MockImageCapture();
    when(() => cap.takePhoto()).thenAnswer((_) async =>
        XFile.fromData(Uint8List.fromList([1, 2, 3]), name: 'p.jpg'));
    var first = true;
    when(() => repo.uploadPage(any(), any())).thenAnswer((_) async {
      if (first) {
        first = false;
        throw Exception('boom');
      }
      return const PageUploadResult(pageNumber: 1, ocrText: 'Recovered');
    });

    final c = _container(repo, cap);
    final notifier = c.read(captureControllerProvider('d1').notifier);
    await notifier.capturePage();
    expect(c.read(captureControllerProvider('d1')).pages.first.status,
        CaptureStatus.failed);

    await notifier.retry(0);
    final p = c.read(captureControllerProvider('d1')).pages.first;
    expect(p.status, CaptureStatus.done);
    expect(p.ocrText, 'Recovered');
  });

  test('rerunOcr updates the page OCR text', () async {
    final repo = MockCaptureRepository();
    final cap = MockImageCapture();
    when(() => cap.takePhoto()).thenAnswer((_) async =>
        XFile.fromData(Uint8List.fromList([1, 2, 3]), name: 'p.jpg'));
    when(() => repo.uploadPage(any(), any())).thenAnswer(
        (_) async => const PageUploadResult(pageNumber: 1, ocrText: 'v1'));
    when(() => repo.rerunOcr(any(), any())).thenAnswer(
        (_) async => const PageUploadResult(pageNumber: 1, ocrText: 'v2'));

    final c = _container(repo, cap);
    final notifier = c.read(captureControllerProvider('d1').notifier);
    await notifier.capturePage();
    await notifier.rerunOcr(1);
    expect(c.read(captureControllerProvider('d1')).pages.first.ocrText, 'v2');
  });
}
