;; Reduce over a large range
(local core (require :io.gitlab.andreyorst.cljlib.core))

(core.reduce core.+ 0 (core.range 1000000))
