import 'dart:typed_data';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/capture/image_capture.dart';
import '../../../core/capture/image_compressor.dart';
import '../data/capture_repository.dart';

enum CaptureStatus { uploading, done, failed }

class CapturedPage {
  const CapturedPage({
    required this.pageNumber,
    required this.ocrText,
    required this.status,
    this.bytes,
  });

  final int pageNumber;
  final String ocrText;
  final CaptureStatus status;
  final Uint8List? bytes;

  CapturedPage copyWith(
      {int? pageNumber, String? ocrText, CaptureStatus? status}) {
    return CapturedPage(
      pageNumber: pageNumber ?? this.pageNumber,
      ocrText: ocrText ?? this.ocrText,
      status: status ?? this.status,
      bytes: bytes,
    );
  }
}

class CaptureState {
  const CaptureState({this.pages = const [], this.capturing = false});
  final List<CapturedPage> pages;
  final bool capturing;

  CaptureState copyWith({List<CapturedPage>? pages, bool? capturing}) {
    return CaptureState(
        pages: pages ?? this.pages, capturing: capturing ?? this.capturing);
  }
}

class CaptureController extends FamilyNotifier<CaptureState, String> {
  @override
  CaptureState build(String arg) => const CaptureState();

  Future<void> capturePage() async {
    final xfile = await ref.read(imageCaptureProvider).takePhoto();
    if (xfile == null) return;
    state = state.copyWith(capturing: true);
    final raw = await xfile.readAsBytes();
    final bytes = await ref.read(imageCompressorProvider).compress(raw);

    final index = state.pages.length;
    state = state.copyWith(
      capturing: false,
      pages: [
        ...state.pages,
        CapturedPage(
          pageNumber: index + 1,
          ocrText: '',
          status: CaptureStatus.uploading,
          bytes: bytes,
        ),
      ],
    );
    await _upload(index, bytes);
  }

  Future<void> _upload(int index, Uint8List bytes) async {
    try {
      final res =
          await ref.read(captureRepositoryProvider).uploadPage(arg, bytes);
      _replace(
          index,
          (p) => p.copyWith(
                pageNumber: res.pageNumber,
                ocrText: res.ocrText,
                status: CaptureStatus.done,
              ));
    } catch (_) {
      _replace(index, (p) => p.copyWith(status: CaptureStatus.failed));
    }
  }

  Future<void> retry(int index) async {
    if (index < 0 || index >= state.pages.length) return;
    final bytes = state.pages[index].bytes;
    if (bytes == null) return;
    _replace(index, (p) => p.copyWith(status: CaptureStatus.uploading));
    await _upload(index, bytes);
  }

  Future<void> rerunOcr(int pageNumber) async {
    final res =
        await ref.read(captureRepositoryProvider).rerunOcr(arg, pageNumber);
    final index = state.pages.indexWhere((p) => p.pageNumber == pageNumber);
    if (index >= 0) {
      _replace(index,
          (p) => p.copyWith(ocrText: res.ocrText, status: CaptureStatus.done));
    }
  }

  void _replace(int index, CapturedPage Function(CapturedPage) update) {
    final list = [...state.pages];
    list[index] = update(list[index]);
    state = state.copyWith(pages: list);
  }
}

final captureControllerProvider =
    NotifierProvider.family<CaptureController, CaptureState, String>(
        CaptureController.new);
