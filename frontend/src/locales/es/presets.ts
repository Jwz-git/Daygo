export default {
  archetype: {
    label: 'Arquetipo',
    hint: 'Decide en qué edificios y paisaje se convierte esta categoría en el mundo 3D.',
    forge: 'Crear',
    sanctum: 'Pensar',
    nexus: 'Conectar',
    camp: 'Descansar',
    order: 'Ordenar',
  },
  profession: {
    softwareEngineer: {
      name: 'Ingeniería de software',
      categories: {
        coding: { name: 'Programación / depuración', details: 'Escribir, refactorizar y corregir código en un IDE o la terminal' },
        codeReview: { name: 'Revisión de código', details: 'Revisar PR, leer diffs y dejar comentarios' },
        research: { name: 'Investigación', details: 'Leer documentación y Stack Overflow, explorar herramientas y API, y escribir documentos de diseño o especificaciones técnicas' },
        communication: { name: 'Comunicación', details: 'Reuniones, reuniones diarias, Slack, correo, videollamadas, mensajería y sincronizaciones' },
      },
    },
    founderExecutive: {
      name: 'Fundación / dirección',
      categories: {
        building: { name: 'Ingeniería / producto', details: 'Programar, diseñar, lanzar funciones y construir con las propias manos' },
        strategy: { name: 'Investigación y estrategia', details: 'Análisis de la competencia, posicionamiento, reflexión a fondo y preparación para inversores' },
        insights: { name: 'Datos y análisis', details: 'Paneles, datos de retención, embudos y finanzas' },
        communication: { name: 'Comunicación', details: 'Reuniones de equipo, llamadas con inversores, demostraciones a usuarios y contratación' },
      },
    },
    designer: {
      name: 'Diseño',
      categories: {
        design: { name: 'Diseño', details: 'Prototipos, componentes de interfaz, flujos de usuario, diseño visual y especificaciones de entrega' },
        research: { name: 'Investigación', details: 'Explorar patrones, auditorías de la competencia, estudios con usuarios y revisión de métricas' },
        communication: { name: 'Comunicación', details: 'Revisiones de diseño, reuniones diarias, sesiones de crítica y presentación de conceptos' },
      },
    },
    student: {
      name: 'Estudiante',
      categories: {
        studying: { name: 'Estudio', details: 'Clases, lecturas, repaso de diapositivas, tarjetas de memoria y material del curso' },
        assignments: { name: 'Tareas', details: 'Trabajos, ejercicios, proyectos de programación e informes de laboratorio' },
        communication: { name: 'Comunicación', details: 'Grupos de estudio, tutorías, chats de grupo y correos a profesores' },
      },
    },
    productManager: {
      name: 'Gestión de producto',
      categories: {
        specs: { name: 'Especificaciones y planificación', details: 'PRD, hojas de ruta, depuración del backlog, planificación de sprints y tickets' },
        research: { name: 'Investigación y análisis', details: 'Investigación de usuarios, revisión de métricas, análisis de la competencia y pruebas A/B' },
        communication: { name: 'Comunicación', details: 'Reuniones diarias, sincronización con interesados, revisiones de diseño y puntos con ingeniería' },
      },
    },
    dataScientist: {
      name: 'Ciencia de datos',
      categories: {
        analysis: { name: 'Análisis y modelado', details: 'Notebooks, análisis estadístico, entrenamiento de modelos y exploración de datos' },
        dataEngineering: { name: 'Ingeniería de datos', details: 'Consultas SQL, pipelines, limpieza de datos y scripts ETL' },
        research: { name: 'Investigación', details: 'Leer artículos y documentación, explorar nuevos métodos y herramientas' },
        communication: { name: 'Comunicación', details: 'Presentar resultados, sincronización con interesados y debates del equipo' },
      },
    },
    other: {
      name: 'Otro',
      categories: {
        work: { name: 'Trabajo', details: 'Trabajo concentrado y responsabilidades profesionales que no encajan en una categoría más específica' },
        communication: { name: 'Comunicación', details: 'Reuniones, reuniones diarias, Slack, correo, videollamadas, mensajería y sincronizaciones' },
      },
    },
  },
  common: {
    distraction: { name: 'Distracción', details: 'Navegación sin rumbo y consumo pasivo de contenido: redes sociales, vídeos al azar, desplazamiento sin objetivo, entretenimiento sin una intención clara y videojuegos' },
    personal: { name: 'Personal', details: 'Actividad no laboral con un propósito claro: hablar con amigos y familiares, gestionar las finanzas, reservar viajes, recados, tareas de la vida diaria y aficiones' },
  },
}
