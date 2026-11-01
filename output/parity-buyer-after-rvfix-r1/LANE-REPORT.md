| Finding | Class | Change |
|---|---|---|
| CI f69c135f RED on the 4 new `shipped card order` cases (expected, tests-only push) | a | OrderStatusView.tsx: shippedPay read (readOrderPayment) + loading gate; OrderTerminalView.tsx: `payment` prop, 已付款 only for CAPTURED without refund activity, refund-processing / refund-succeeded lines from payment copy |

Not run locally (owner rule); CI is the verdict. WAIT_RUN

STATUS: DONE
