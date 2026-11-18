      *================================================================
      * CTAREG - REGISTRO MAESTRO DE CUENTAS
      * CLAVE RELATIVA = NUMERO DE CUENTA
      *================================================================
       01  CTA-REGISTRO.
           05  CTA-NUMERO          PIC 9(10).
           05  CTA-CLIENTE         PIC 9(8).
           05  CTA-TITULAR         PIC X(30).
           05  CTA-TIPO            PIC X.
               88  CTA-CORRIENTE   VALUE 'C'.
               88  CTA-AHORRO      VALUE 'A'.
               88  CTA-PLAZO-FIJO  VALUE 'P'.
               88  CTA-PRESTAMO    VALUE 'L'.
           05  CTA-SALDO           PIC S9(11)V99 COMP-3.
           05  CTA-LIMITE-DESC     PIC 9(9)V99.
           05  CTA-ESTADO          PIC X.
               88  CTA-ACTIVA      VALUE 'A'.
               88  CTA-BLOQUEADA   VALUE 'B'.
               88  CTA-CERRADA     VALUE 'C'.
               88  CTA-EMBARGADA   VALUE 'E'.
           05  CTA-FECHA-ALTA      PIC 9(8).
           05  CTA-FECHA-VENC      PIC 9(8).
           05  CTA-DIAS-MORA       PIC 9(4).
           05  CTA-COD-TASA        PIC 99.
           05  CTA-MOVS-MES        PIC 9(5).
