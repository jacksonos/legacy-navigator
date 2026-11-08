      * REGISTRO DE LA CUENTA
       01  CTA-REGISTRO.
           05  CTA-NUMERO          PIC 9(10).
           05  CTA-TITULAR         PIC X(30).
           05  CTA-SALDO           PIC S9(11)V99 COMP-3.
           05  CTA-ESTADO          PIC X.
               88  CTA-ACTIVA      VALUE 'A'.
               88  CTA-BLOQUEADA   VALUE 'B'.
               88  CTA-CERRADA     VALUE 'C'.
