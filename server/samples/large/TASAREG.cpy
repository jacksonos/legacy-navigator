      *================================================================
      * TASAREG - TABLA DE TASAS DE INTERES Y COMISIONES POR PRODUCTO
      * CADA REGISTRO TIENE HASTA 5 TRAMOS DE SALDO CON SU TASA ANUAL
      *================================================================
       01  TASA-REGISTRO.
           05  TASA-CODIGO         PIC 99.
           05  TASA-DESCRIPCION    PIC X(20).
           05  TASA-NUM-TRAMOS     PIC 9.
           05  TASA-TRAMO OCCURS 5 TIMES.
               10  TASA-DESDE      PIC 9(11)V99.
               10  TASA-PORC-ANUAL PIC 9V9(4).
           05  TASA-COMISION-FIJA  PIC 9(5)V99.
           05  TASA-PORC-MORA      PIC 9V9(4).
