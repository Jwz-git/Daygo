export default {
  discardNote: 'Les modifications non enregistrées seront perdues.',
  apply: 'Appliquer',
  cancel: 'Annuler',
  applying: 'Application…',
  failed: 'Impossible d’appliquer. Tes catégories n’ont pas changé.',
  palette: {
    title: 'Palettes de couleurs',
    hint: 'Choisis une palette pour recolorer toutes les catégories dans l’ordre.',
    confirmBody: 'Recolore toutes les catégories avec « {name} ».',
    names: {
      daygoClassic: 'Daygo classique',
      forestMoss: 'Mousse forestière',
      cyberNeon: 'Néon cyber',
      nordicMist: 'Brume nordique',
      sunsetClay: 'Argile du couchant',
    },
  },
  archetype: {
    label: 'Archétype',
    hint: 'Détermine les bâtiments et le paysage que devient cette catégorie dans le monde 3D.',
    forge: 'Créer',
    sanctum: 'Réfléchir',
    nexus: 'Relier',
    camp: 'Se reposer',
    order: 'Organiser',
  },
  profession: {
    softwareEngineer: {
      name: 'Développement logiciel',
      categories: {
        coding: { name: 'Code / débogage', details: 'Écrire, refactoriser et corriger du code dans un IDE ou un terminal' },
        codeReview: { name: 'Revue de code', details: 'Relire des PR, lire des diffs et laisser des commentaires' },
        research: { name: 'Recherche', details: 'Lire la documentation et Stack Overflow, explorer outils et API, rédiger des documents de conception ou des spécifications techniques' },
        communication: { name: 'Communication', details: 'Réunions, mêlées, Slack, e-mails, appels vidéo, messagerie et points de synchronisation' },
      },
    },
    founderExecutive: {
      name: 'Fondation / direction',
      categories: {
        building: { name: 'Ingénierie / produit', details: 'Coder, concevoir, livrer des fonctionnalités et construire soi-même' },
        strategy: { name: 'Recherche et stratégie', details: 'Veille concurrentielle, positionnement, réflexion de fond et préparation des investisseurs' },
        insights: { name: 'Données et analyses', details: 'Tableaux de bord, rétention, entonnoirs et finances' },
        communication: { name: 'Communication', details: 'Points d’équipe, appels avec les investisseurs, démos utilisateurs et recrutement' },
      },
    },
    designer: {
      name: 'Design',
      categories: {
        design: { name: 'Conception', details: 'Prototypes, composants d’interface, parcours utilisateurs, design visuel et spécifications de livraison' },
        research: { name: 'Recherche', details: 'Repérer des modèles, audits concurrentiels, études utilisateurs et suivi des indicateurs' },
        communication: { name: 'Communication', details: 'Revues de design, points quotidiens, séances de critique et présentation de concepts' },
      },
    },
    student: {
      name: 'Études',
      categories: {
        studying: { name: 'Étude', details: 'Cours, lectures, révision de diapositives, fiches et supports de cours' },
        assignments: { name: 'Devoirs', details: 'Dissertations, séries d’exercices, projets de programmation et comptes rendus de TP' },
        communication: { name: 'Communication', details: 'Groupes de travail, permanences, discussions de groupe et e-mails aux enseignants' },
      },
    },
    productManager: {
      name: 'Gestion de produit',
      categories: {
        specs: { name: 'Specs et planification', details: 'PRD, feuilles de route, tri du backlog, planification de sprint et tickets' },
        research: { name: 'Recherche et analyse', details: 'Études utilisateurs, suivi des indicateurs, analyse concurrentielle et tests A/B' },
        communication: { name: 'Communication', details: 'Points quotidiens, échanges avec les parties prenantes, revues de design et points avec l’ingénierie' },
      },
    },
    dataScientist: {
      name: 'Science des données',
      categories: {
        analysis: { name: 'Analyse et modélisation', details: 'Notebooks, analyses statistiques, entraînement de modèles et exploration de données' },
        dataEngineering: { name: 'Ingénierie des données', details: 'Requêtes SQL, pipelines, nettoyage de données et scripts ETL' },
        research: { name: 'Recherche', details: 'Lire des articles et de la documentation, explorer de nouvelles méthodes et de nouveaux outils' },
        communication: { name: 'Communication', details: 'Présenter des résultats, échanges avec les parties prenantes et discussions d’équipe' },
      },
    },
    other: {
      name: 'Autre',
      categories: {
        work: { name: 'Travail', details: 'Tâches de fond et responsabilités professionnelles qui n’entrent dans aucune catégorie plus précise' },
        communication: { name: 'Communication', details: 'Réunions, mêlées, Slack, e-mails, appels vidéo, messagerie et points de synchronisation' },
      },
    },
  },
  common: {
    distraction: { name: 'Distraction', details: 'Navigation sans but et consommation passive de contenu : fils de réseaux sociaux, vidéos au hasard, défilement sans intention, divertissement sans objectif clair et jeux' },
    personal: { name: 'Personnel', details: 'Activités non professionnelles menées volontairement : échanger avec ses proches, gérer ses finances, réserver un voyage, courses, tâches administratives et loisirs' },
  },
}
