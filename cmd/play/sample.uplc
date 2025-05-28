-- The bytestring is the compressed version of the G2 zero point, but extended to 97 bytes.
(program 1.0.0
  [
    [
      (builtin shiftByteString)
      (con bytestring #ebfc)
    ]
    (con integer 5)
  ]
)
