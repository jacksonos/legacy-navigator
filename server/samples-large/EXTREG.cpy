      *================================================================
      * EXTREG - LINEAS DEL EXTRACTO MENSUAL (IMPRESION DE 132 COL)
      *================================================================
       01  EXT-CABECERA.
           05  FILLER              PIC X(20) VALUE 'BANCO EJEMPLO S.A.'.
           05  FILLER              PIC X(30) VALUE
               'EXTRACTO MENSUAL DE CUENTA'.
           05  FILLER              PIC X(8)  VALUE 'PAGINA: '.
           05  EXT-CAB-PAGINA      PIC ZZZ9.
       01  EXT-TITULAR-LINEA.
           05  FILLER              PIC X(9)  VALUE 'CUENTA:  '.
           05  EXT-TIT-CUENTA      PIC 9(10).
           05  FILLER              PIC X(3)  VALUE SPACES.
           05  EXT-TIT-NOMBRE      PIC X(30).
       01  EXT-DETALLE.
           05  EXT-DET-FECHA       PIC 9(8).
           05  FILLER              PIC X(2)  VALUE SPACES.
           05  EXT-DET-CONCEPTO    PIC X(20).
           05  EXT-DET-IMPORTE     PIC -(9)9.99.
           05  FILLER              PIC X(2)  VALUE SPACES.
           05  EXT-DET-SALDO       PIC -(11)9.99.
       01  EXT-TOTAL-LINEA.
           05  FILLER              PIC X(20) VALUE 'SALDO FINAL:'.
           05  EXT-TOT-SALDO       PIC -(11)9.99.
