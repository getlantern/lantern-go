#lang web-server/insta
 
(define buf (bytes->string/utf-8 (make-bytes (* 1024 1024) 100)))

(define (start req)
  (response/xexpr buf))