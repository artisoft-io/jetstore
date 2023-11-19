import 'package:flutter/material.dart';

class JetsSpinnerOverlay extends StatefulWidget {
  const JetsSpinnerOverlay({
    Key? key,
    required this.child,
    this.delay = const Duration(milliseconds: 500),
  }) : super(key: key);

  final Widget child;
  final Duration delay;

  static JetsSpinnerOverlayState of(BuildContext context) {
    return context.findAncestorStateOfType<JetsSpinnerOverlayState>()!;
  }

  @override
  State<JetsSpinnerOverlay> createState() => JetsSpinnerOverlayState();
}

class JetsSpinnerOverlayState extends State<JetsSpinnerOverlay> {
  bool _isLoading = false;
  bool get isLoading => _isLoading;

  void show() {
    setState(() {
      _isLoading = true;
    });
  }

  void hide() {
    setState(() {
      _isLoading = false;
    });
  }

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: [
        widget.child,
        if (_isLoading)
          const Opacity(
            opacity: 0.2,
            child: ModalBarrier(dismissible: false, color: Colors.black),
          ),
        if (_isLoading)
          Center(
            child: FutureBuilder(
              future: Future.delayed(widget.delay),
              builder: (context, snapshot) {
                return snapshot.connectionState == ConnectionState.done
                    ? const CircularProgressIndicator()
                    : const SizedBox();
              },
            ),
          ),
      ],
    );
  }
}
