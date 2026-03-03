--
-- PostgreSQL database dump
--

\restrict FbrFbliVx4C1sfrmuGDzAjgF19SJOtBinUEfmorsBJ0kvycUPtlNtoczc2dOkuY

-- Dumped from database version 16.11
-- Dumped by pg_dump version 16.11

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: public; Type: SCHEMA; Schema: -; Owner: anigraph
--

-- *not* creating schema, since initdb creates it


ALTER SCHEMA public OWNER TO anigraph;

--
-- Name: SCHEMA public; Type: COMMENT; Schema: -; Owner: anigraph
--

COMMENT ON SCHEMA public IS '';


--
-- Name: pg_trgm; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;


--
-- Name: EXTENSION pg_trgm; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION pg_trgm IS 'text similarity measurement and index searching based on trigrams';


--
-- Name: cleanup_expired_sessions(); Type: FUNCTION; Schema: public; Owner: anigraph
--

CREATE FUNCTION public.cleanup_expired_sessions() RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
    DELETE FROM user_sessions WHERE expires_at < CURRENT_TIMESTAMP;
END;
$$;


ALTER FUNCTION public.cleanup_expired_sessions() OWNER TO anigraph;

--
-- Name: cleanup_list_cache(); Type: FUNCTION; Schema: public; Owner: anigraph
--

CREATE FUNCTION public.cleanup_list_cache() RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
  DELETE FROM list_computation_cache
  WHERE id NOT IN (
    SELECT id FROM list_computation_cache
    ORDER BY last_accessed DESC
    LIMIT 100
  );
END;
$$;


ALTER FUNCTION public.cleanup_list_cache() OWNER TO anigraph;

--
-- Name: populate_all_denormalized_arrays(); Type: FUNCTION; Schema: public; Owner: anigraph
--

CREATE FUNCTION public.populate_all_denormalized_arrays() RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
    -- Populate genre_names
    UPDATE anime a SET genre_names = (
        SELECT array_agg(g.name ORDER BY g.name)
        FROM anime_genre ag
        JOIN genre g ON ag.genre_id = g.id
        WHERE ag.anime_id = a.id
    );

    -- Populate tag_names
    UPDATE anime a SET tag_names = (
        SELECT array_agg(t.name ORDER BY t.name)
        FROM anime_tag at
        JOIN tag t ON at.tag_id = t.id
        WHERE at.anime_id = a.id
    );

    -- Populate studio_names
    UPDATE anime a SET studio_names = (
        SELECT array_agg(s.name ORDER BY s.name)
        FROM anime_studio ast
        JOIN studio s ON ast.studio_id = s.id
        WHERE ast.anime_id = a.id
    );
END;
$$;


ALTER FUNCTION public.populate_all_denormalized_arrays() OWNER TO anigraph;

--
-- Name: refresh_anime_genre_names(integer); Type: FUNCTION; Schema: public; Owner: anigraph
--

CREATE FUNCTION public.refresh_anime_genre_names(p_anime_id integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
    UPDATE anime SET genre_names = (
        SELECT array_agg(g.name ORDER BY g.name)
        FROM anime_genre ag
        JOIN genre g ON ag.genre_id = g.id
        WHERE ag.anime_id = p_anime_id
    )
    WHERE id = p_anime_id;
END;
$$;


ALTER FUNCTION public.refresh_anime_genre_names(p_anime_id integer) OWNER TO anigraph;

--
-- Name: refresh_anime_studio_names(integer); Type: FUNCTION; Schema: public; Owner: anigraph
--

CREATE FUNCTION public.refresh_anime_studio_names(p_anime_id integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
    UPDATE anime SET studio_names = (
        SELECT array_agg(s.name ORDER BY s.name)
        FROM anime_studio ast
        JOIN studio s ON ast.studio_id = s.id
        WHERE ast.anime_id = p_anime_id
    )
    WHERE id = p_anime_id;
END;
$$;


ALTER FUNCTION public.refresh_anime_studio_names(p_anime_id integer) OWNER TO anigraph;

--
-- Name: refresh_anime_tag_names(integer); Type: FUNCTION; Schema: public; Owner: anigraph
--

CREATE FUNCTION public.refresh_anime_tag_names(p_anime_id integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
    UPDATE anime SET tag_names = (
        SELECT array_agg(t.name ORDER BY t.name)
        FROM anime_tag at
        JOIN tag t ON at.tag_id = t.id
        WHERE at.anime_id = p_anime_id
    )
    WHERE id = p_anime_id;
END;
$$;


ALTER FUNCTION public.refresh_anime_tag_names(p_anime_id integer) OWNER TO anigraph;

--
-- Name: refresh_random_ranks(); Type: FUNCTION; Schema: public; Owner: anigraph
--

CREATE FUNCTION public.refresh_random_ranks() RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
    UPDATE anime SET random_rank = random();
END;
$$;


ALTER FUNCTION public.refresh_random_ranks() OWNER TO anigraph;

--
-- Name: sync_anime_genre_names(); Type: FUNCTION; Schema: public; Owner: anigraph
--

CREATE FUNCTION public.sync_anime_genre_names() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF TG_OP = 'INSERT' OR TG_OP = 'UPDATE' THEN
        PERFORM refresh_anime_genre_names(NEW.anime_id);
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        PERFORM refresh_anime_genre_names(OLD.anime_id);
        RETURN OLD;
    END IF;
END;
$$;


ALTER FUNCTION public.sync_anime_genre_names() OWNER TO anigraph;

--
-- Name: sync_anime_studio_names(); Type: FUNCTION; Schema: public; Owner: anigraph
--

CREATE FUNCTION public.sync_anime_studio_names() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF TG_OP = 'INSERT' OR TG_OP = 'UPDATE' THEN
        PERFORM refresh_anime_studio_names(NEW.anime_id);
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        PERFORM refresh_anime_studio_names(OLD.anime_id);
        RETURN OLD;
    END IF;
END;
$$;


ALTER FUNCTION public.sync_anime_studio_names() OWNER TO anigraph;

--
-- Name: sync_anime_tag_names(); Type: FUNCTION; Schema: public; Owner: anigraph
--

CREATE FUNCTION public.sync_anime_tag_names() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF TG_OP = 'INSERT' OR TG_OP = 'UPDATE' THEN
        PERFORM refresh_anime_tag_names(NEW.anime_id);
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        PERFORM refresh_anime_tag_names(OLD.anime_id);
        RETURN OLD;
    END IF;
END;
$$;


ALTER FUNCTION public.sync_anime_tag_names() OWNER TO anigraph;

--
-- Name: update_genre_anime_count(); Type: FUNCTION; Schema: public; Owner: anigraph
--

CREATE FUNCTION public.update_genre_anime_count() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE genre SET anime_count = anime_count + 1 WHERE id = NEW.genre_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE genre SET anime_count = anime_count - 1 WHERE id = OLD.genre_id;
    END IF;
    RETURN NULL;
END;
$$;


ALTER FUNCTION public.update_genre_anime_count() OWNER TO anigraph;

--
-- Name: update_studio_anime_count(); Type: FUNCTION; Schema: public; Owner: anigraph
--

CREATE FUNCTION public.update_studio_anime_count() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE studio SET anime_count = anime_count + 1 WHERE id = NEW.studio_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE studio SET anime_count = anime_count - 1 WHERE id = OLD.studio_id;
    END IF;
    RETURN NULL;
END;
$$;


ALTER FUNCTION public.update_studio_anime_count() OWNER TO anigraph;

--
-- Name: update_tag_anime_count(); Type: FUNCTION; Schema: public; Owner: anigraph
--

CREATE FUNCTION public.update_tag_anime_count() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE tag SET anime_count = anime_count + 1 WHERE id = NEW.tag_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE tag SET anime_count = anime_count - 1 WHERE id = OLD.tag_id;
    END IF;
    RETURN NULL;
END;
$$;


ALTER FUNCTION public.update_tag_anime_count() OWNER TO anigraph;

--
-- Name: update_taste_profile_favorite_count(); Type: FUNCTION; Schema: public; Owner: anigraph
--

CREATE FUNCTION public.update_taste_profile_favorite_count() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        -- Check if profile exists
        IF EXISTS (SELECT 1 FROM user_taste_profiles WHERE user_id = NEW.user_id AND list_id IS NULL) THEN
            UPDATE user_taste_profiles
            SET total_favorites = total_favorites + 1
            WHERE user_id = NEW.user_id AND list_id IS NULL;
        ELSE
            INSERT INTO user_taste_profiles (user_id, list_id, total_favorites)
            VALUES (NEW.user_id, NULL, 1);
        END IF;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE user_taste_profiles
        SET total_favorites = total_favorites - 1
        WHERE user_id = OLD.user_id AND list_id IS NULL;
    END IF;
    RETURN NULL;
END;
$$;


ALTER FUNCTION public.update_taste_profile_favorite_count() OWNER TO anigraph;

--
-- Name: update_user_last_active(); Type: FUNCTION; Schema: public; Owner: anigraph
--

CREATE FUNCTION public.update_user_last_active() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    UPDATE users SET last_active = CURRENT_TIMESTAMP WHERE user_id = NEW.user_id;
    RETURN NEW;
END;
$$;


ALTER FUNCTION public.update_user_last_active() OWNER TO anigraph;

--
-- Name: update_user_list_timestamp(); Type: FUNCTION; Schema: public; Owner: anigraph
--

CREATE FUNCTION public.update_user_list_timestamp() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;


ALTER FUNCTION public.update_user_list_timestamp() OWNER TO anigraph;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: anime; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.anime (
    id integer NOT NULL,
    anilist_id integer NOT NULL,
    title character varying(500),
    title_english character varying(500),
    title_romaji character varying(500),
    title_native character varying(500),
    title_ja character varying(500),
    synonyms text[],
    type character varying(20),
    format character varying(20),
    status character varying(30),
    source character varying(30),
    season character varying(10),
    season_year integer,
    start_date date,
    end_date date,
    episodes integer,
    duration integer,
    average_score numeric(5,2),
    mean_score numeric(5,2),
    popularity integer,
    favourites integer,
    trending integer,
    description text,
    country_of_origin character varying(10),
    is_adult boolean DEFAULT false,
    cover_image character varying(500),
    cover_image_extra_large character varying(500),
    cover_image_large character varying(500),
    cover_image_medium character varying(500),
    cover_image_color character varying(20),
    banner_image character varying(500),
    keyframe_link character varying(500),
    trailer_id character varying(200),
    trailer_site character varying(50),
    trailer_thumbnail character varying(200),
    rank_overall_format integer,
    rank_year_format integer,
    rank_year_format_genre jsonb,
    community_id integer,
    franchise_id integer,
    genre_names text[],
    tag_names text[],
    studio_names text[],
    updated_at timestamp without time zone,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    random_rank double precision,
    wikidata_qid character varying(20),
    mal_id integer,
    wikidata_searched_at timestamp with time zone,
    wikipedia_en text,
    wikipedia_ja text,
    livechart_id text,
    notify_id text,
    tvdb_id text,
    tmdb_movie_id text,
    tmdb_tv_id text,
    tvmaze_id text,
    mywaifulist_id text,
    unconsenting_media_id text,
    sakugabooru_tag character varying(200),
    wikipedia_production_html text,
    anidb_id integer,
    ann_id integer,
    animeplanet_slug character varying(500),
    anisearch_id integer,
    annict_id integer,
    imdb_id character varying(20),
    kaize_slug character varying(500),
    kitsu_id integer,
    nautiljon_slug character varying(500),
    otakotaku_id integer,
    shikimori_id integer,
    shoboi_id integer,
    silveryasha_id integer,
    simkl_id integer,
    trakt_id integer,
    trakt_type character varying(10),
    trakt_season integer
);


ALTER TABLE public.anime OWNER TO anigraph;

--
-- Name: anime_genre; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.anime_genre (
    anime_id integer NOT NULL,
    genre_id integer NOT NULL
);


ALTER TABLE public.anime_genre OWNER TO anigraph;

--
-- Name: anime_studio; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.anime_studio (
    anime_id integer NOT NULL,
    studio_id integer NOT NULL,
    is_main boolean DEFAULT false
);


ALTER TABLE public.anime_studio OWNER TO anigraph;

--
-- Name: anime_tag; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.anime_tag (
    anime_id integer NOT NULL,
    tag_id integer NOT NULL,
    rank integer,
    score integer
);


ALTER TABLE public.anime_tag OWNER TO anigraph;

--
-- Name: anime_filter_metadata; Type: VIEW; Schema: public; Owner: anigraph
--

CREATE VIEW public.anime_filter_metadata AS
 SELECT a.id,
    a.anilist_id,
    array_agg(DISTINCT ast.studio_id) FILTER (WHERE (ast.studio_id IS NOT NULL)) AS studio_ids,
    array_agg(DISTINCT ag.genre_id) FILTER (WHERE (ag.genre_id IS NOT NULL)) AS genre_ids,
    array_agg(DISTINCT at.tag_id) FILTER (WHERE (at.tag_id IS NOT NULL)) AS tag_ids
   FROM (((public.anime a
     LEFT JOIN public.anime_studio ast ON ((a.id = ast.anime_id)))
     LEFT JOIN public.anime_genre ag ON ((a.id = ag.anime_id)))
     LEFT JOIN public.anime_tag at ON ((a.id = at.anime_id)))
  GROUP BY a.id, a.anilist_id;


ALTER VIEW public.anime_filter_metadata OWNER TO anigraph;

--
-- Name: anime_graph_cache; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.anime_graph_cache (
    anilist_id integer NOT NULL,
    graph_data jsonb NOT NULL
);


ALTER TABLE public.anime_graph_cache OWNER TO anigraph;

--
-- Name: anime_id_seq; Type: SEQUENCE; Schema: public; Owner: anigraph
--

CREATE SEQUENCE public.anime_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.anime_id_seq OWNER TO anigraph;

--
-- Name: anime_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: anigraph
--

ALTER SEQUENCE public.anime_id_seq OWNED BY public.anime.id;


--
-- Name: anime_recommendation; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.anime_recommendation (
    anime_id integer NOT NULL,
    recommended_anime_id integer NOT NULL,
    similarity numeric(10,6),
    rank integer
);


ALTER TABLE public.anime_recommendation OWNER TO anigraph;

--
-- Name: anime_relation; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.anime_relation (
    anime_id integer NOT NULL,
    related_anime_id integer NOT NULL,
    relation_type character varying(30) NOT NULL,
    relation_type_rank integer
);


ALTER TABLE public.anime_relation OWNER TO anigraph;

--
-- Name: anime_sakugabooru_post; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.anime_sakugabooru_post (
    anime_id integer NOT NULL,
    post_id integer NOT NULL
);


ALTER TABLE public.anime_sakugabooru_post OWNER TO anigraph;

--
-- Name: anime_staff; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.anime_staff (
    anime_id integer NOT NULL,
    staff_id integer NOT NULL,
    role text[],
    weight numeric(10,4)
);


ALTER TABLE public.anime_staff OWNER TO anigraph;

--
-- Name: studio; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.studio (
    id integer NOT NULL,
    name character varying(200) NOT NULL,
    anime_count integer DEFAULT 0,
    mal_id integer,
    image_url character varying(500),
    description text,
    wikidata_qid character varying(20),
    wikipedia_en character varying(500),
    wikipedia_ja character varying(500),
    website_url character varying(500),
    twitter_handle character varying(100),
    youtube_channel_id character varying(100),
    wikipedia_content_html text
);


ALTER TABLE public.studio OWNER TO anigraph;

--
-- Name: anime_with_main_studio; Type: VIEW; Schema: public; Owner: anigraph
--

CREATE VIEW public.anime_with_main_studio AS
 SELECT a.id,
    a.anilist_id,
    a.title,
    a.title_english,
    a.title_romaji,
    a.title_native,
    a.title_ja,
    a.synonyms,
    a.type,
    a.format,
    a.status,
    a.source,
    a.season,
    a.season_year,
    a.start_date,
    a.end_date,
    a.episodes,
    a.duration,
    a.average_score,
    a.mean_score,
    a.popularity,
    a.favourites,
    a.trending,
    a.description,
    a.country_of_origin,
    a.is_adult,
    a.cover_image,
    a.cover_image_extra_large,
    a.cover_image_large,
    a.cover_image_medium,
    a.cover_image_color,
    a.banner_image,
    a.keyframe_link,
    a.trailer_id,
    a.trailer_site,
    a.trailer_thumbnail,
    a.rank_overall_format,
    a.rank_year_format,
    a.rank_year_format_genre,
    a.community_id,
    a.franchise_id,
    a.genre_names,
    a.tag_names,
    a.studio_names,
    a.updated_at,
    a.created_at,
    s.name AS main_studio_name
   FROM ((public.anime a
     LEFT JOIN public.anime_studio ast ON (((a.id = ast.anime_id) AND (ast.is_main = true))))
     LEFT JOIN public.studio s ON ((ast.studio_id = s.id)));


ALTER VIEW public.anime_with_main_studio OWNER TO anigraph;

--
-- Name: audit_logs; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.audit_logs (
    id integer NOT NULL,
    user_id uuid NOT NULL,
    action character varying(100) NOT NULL,
    resource_type character varying(50) NOT NULL,
    resource_id character varying(255) NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb,
    ip_address character varying(45),
    user_agent text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.audit_logs OWNER TO anigraph;

--
-- Name: TABLE audit_logs; Type: COMMENT; Schema: public; Owner: anigraph
--

COMMENT ON TABLE public.audit_logs IS 'Audit trail for sensitive operations (compliance, security monitoring)';


--
-- Name: COLUMN audit_logs.action; Type: COMMENT; Schema: public; Owner: anigraph
--

COMMENT ON COLUMN public.audit_logs.action IS 'Dot-notation action type (e.g., list.create, list.privacy_change, user.merge)';


--
-- Name: COLUMN audit_logs.metadata; Type: COMMENT; Schema: public; Owner: anigraph
--

COMMENT ON COLUMN public.audit_logs.metadata IS 'Additional context about the action (JSON)';


--
-- Name: audit_logs_id_seq; Type: SEQUENCE; Schema: public; Owner: anigraph
--

CREATE SEQUENCE public.audit_logs_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.audit_logs_id_seq OWNER TO anigraph;

--
-- Name: audit_logs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: anigraph
--

ALTER SEQUENCE public.audit_logs_id_seq OWNED BY public.audit_logs.id;


--
-- Name: franchise; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.franchise (
    id integer NOT NULL,
    title character varying(500) NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.franchise OWNER TO anigraph;

--
-- Name: franchise_id_seq; Type: SEQUENCE; Schema: public; Owner: anigraph
--

CREATE SEQUENCE public.franchise_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.franchise_id_seq OWNER TO anigraph;

--
-- Name: franchise_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: anigraph
--

ALTER SEQUENCE public.franchise_id_seq OWNED BY public.franchise.id;


--
-- Name: genre; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.genre (
    id integer NOT NULL,
    name character varying(100) NOT NULL,
    description text,
    anime_count integer DEFAULT 0
);


ALTER TABLE public.genre OWNER TO anigraph;

--
-- Name: genre_id_seq; Type: SEQUENCE; Schema: public; Owner: anigraph
--

CREATE SEQUENCE public.genre_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.genre_id_seq OWNER TO anigraph;

--
-- Name: genre_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: anigraph
--

ALTER SEQUENCE public.genre_id_seq OWNED BY public.genre.id;


--
-- Name: list_computation_cache; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.list_computation_cache (
    id integer NOT NULL,
    cache_key character varying(64) NOT NULL,
    anime_ids integer[] NOT NULL,
    taste_profile jsonb,
    recommendations jsonb,
    last_accessed timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.list_computation_cache OWNER TO anigraph;

--
-- Name: list_computation_cache_id_seq; Type: SEQUENCE; Schema: public; Owner: anigraph
--

CREATE SEQUENCE public.list_computation_cache_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.list_computation_cache_id_seq OWNER TO anigraph;

--
-- Name: list_computation_cache_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: anigraph
--

ALTER SEQUENCE public.list_computation_cache_id_seq OWNED BY public.list_computation_cache.id;


--
-- Name: sakugabooru_post; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.sakugabooru_post (
    post_id integer NOT NULL,
    file_url character varying(500) NOT NULL,
    preview_url character varying(500),
    rating character varying(1) DEFAULT 's'::character varying,
    source text,
    fetched_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    file_ext character varying(10)
);


ALTER TABLE public.sakugabooru_post OWNER TO anigraph;

--
-- Name: staff; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.staff (
    id integer NOT NULL,
    staff_id integer NOT NULL,
    name_en character varying(200),
    name_ja character varying(200),
    pen_name_en character varying(200),
    pen_name_ja character varying(200),
    image_large character varying(500),
    image_medium character varying(500),
    language character varying(50),
    description text,
    primary_occupations text[],
    gender character varying(20),
    date_of_birth_year integer,
    date_of_birth_month integer,
    date_of_birth_day integer,
    date_of_death_year integer,
    date_of_death_month integer,
    date_of_death_day integer,
    age integer,
    years_active integer[],
    home_town character varying(200),
    blood_type character varying(10),
    community_id integer,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    sakugabooru_tag character varying(200),
    alternative_names text[]
);


ALTER TABLE public.staff OWNER TO anigraph;

--
-- Name: staff_id_seq; Type: SEQUENCE; Schema: public; Owner: anigraph
--

CREATE SEQUENCE public.staff_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.staff_id_seq OWNER TO anigraph;

--
-- Name: staff_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: anigraph
--

ALTER SEQUENCE public.staff_id_seq OWNED BY public.staff.id;


--
-- Name: staff_sakugabooru_post; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.staff_sakugabooru_post (
    staff_id integer NOT NULL,
    post_id integer NOT NULL
);


ALTER TABLE public.staff_sakugabooru_post OWNER TO anigraph;

--
-- Name: studio_collaboration; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.studio_collaboration (
    studio_id integer NOT NULL,
    collaborator_id integer NOT NULL,
    shared_anime_count integer DEFAULT 0
);


ALTER TABLE public.studio_collaboration OWNER TO anigraph;

--
-- Name: studio_id_seq; Type: SEQUENCE; Schema: public; Owner: anigraph
--

CREATE SEQUENCE public.studio_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.studio_id_seq OWNER TO anigraph;

--
-- Name: studio_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: anigraph
--

ALTER SEQUENCE public.studio_id_seq OWNED BY public.studio.id;


--
-- Name: tag; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.tag (
    id integer NOT NULL,
    name character varying(200) NOT NULL,
    category character varying(100),
    description text,
    anime_count integer DEFAULT 0
);


ALTER TABLE public.tag OWNER TO anigraph;

--
-- Name: tag_id_seq; Type: SEQUENCE; Schema: public; Owner: anigraph
--

CREATE SEQUENCE public.tag_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tag_id_seq OWNER TO anigraph;

--
-- Name: tag_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: anigraph
--

ALTER SEQUENCE public.tag_id_seq OWNED BY public.tag.id;


--
-- Name: user_anime_predictions; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.user_anime_predictions (
    id integer NOT NULL,
    user_id uuid NOT NULL,
    anime_id integer NOT NULL,
    list_id integer,
    match_score integer NOT NULL,
    genre_score double precision,
    tag_score double precision,
    staff_score double precision,
    studio_score double precision,
    reasons jsonb DEFAULT '[]'::jsonb,
    computed_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT user_anime_predictions_match_score_check CHECK (((match_score >= 0) AND (match_score <= 100)))
);


ALTER TABLE public.user_anime_predictions OWNER TO anigraph;

--
-- Name: user_anime_predictions_id_seq; Type: SEQUENCE; Schema: public; Owner: anigraph
--

CREATE SEQUENCE public.user_anime_predictions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.user_anime_predictions_id_seq OWNER TO anigraph;

--
-- Name: user_anime_predictions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: anigraph
--

ALTER SEQUENCE public.user_anime_predictions_id_seq OWNED BY public.user_anime_predictions.id;


--
-- Name: user_favorites; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.user_favorites (
    id integer NOT NULL,
    user_id uuid NOT NULL,
    anime_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.user_favorites OWNER TO anigraph;

--
-- Name: user_favorites_id_seq; Type: SEQUENCE; Schema: public; Owner: anigraph
--

CREATE SEQUENCE public.user_favorites_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.user_favorites_id_seq OWNER TO anigraph;

--
-- Name: user_favorites_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: anigraph
--

ALTER SEQUENCE public.user_favorites_id_seq OWNED BY public.user_favorites.id;


--
-- Name: user_list_items; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.user_list_items (
    id integer NOT NULL,
    list_id integer NOT NULL,
    anime_id integer NOT NULL,
    notes text,
    added_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.user_list_items OWNER TO anigraph;

--
-- Name: user_list_items_id_seq; Type: SEQUENCE; Schema: public; Owner: anigraph
--

CREATE SEQUENCE public.user_list_items_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.user_list_items_id_seq OWNER TO anigraph;

--
-- Name: user_list_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: anigraph
--

ALTER SEQUENCE public.user_list_items_id_seq OWNED BY public.user_list_items.id;


--
-- Name: user_lists; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.user_lists (
    id integer NOT NULL,
    user_id character varying(255) NOT NULL,
    name character varying(255) NOT NULL,
    description text,
    list_type character varying(20) DEFAULT 'custom'::character varying NOT NULL,
    is_public boolean DEFAULT false,
    share_token character varying(64),
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.user_lists OWNER TO anigraph;

--
-- Name: COLUMN user_lists.list_type; Type: COMMENT; Schema: public; Owner: anigraph
--

COMMENT ON COLUMN public.user_lists.list_type IS 'Type of list: custom or favorites (system list)';


--
-- Name: user_lists_id_seq; Type: SEQUENCE; Schema: public; Owner: anigraph
--

CREATE SEQUENCE public.user_lists_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.user_lists_id_seq OWNER TO anigraph;

--
-- Name: user_lists_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: anigraph
--

ALTER SEQUENCE public.user_lists_id_seq OWNED BY public.user_lists.id;


--
-- Name: user_preferences; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.user_preferences (
    user_id uuid NOT NULL,
    theme_id character varying(50) DEFAULT 'midnight'::character varying NOT NULL,
    include_adult boolean DEFAULT false NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.user_preferences OWNER TO anigraph;

--
-- Name: user_sessions; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.user_sessions (
    id integer NOT NULL,
    session_id character varying(255) NOT NULL,
    user_id uuid NOT NULL,
    data jsonb DEFAULT '{}'::jsonb,
    expires_at timestamp without time zone NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.user_sessions OWNER TO anigraph;

--
-- Name: TABLE user_sessions; Type: COMMENT; Schema: public; Owner: anigraph
--

COMMENT ON TABLE public.user_sessions IS 'OAuth session management';


--
-- Name: user_sessions_id_seq; Type: SEQUENCE; Schema: public; Owner: anigraph
--

CREATE SEQUENCE public.user_sessions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.user_sessions_id_seq OWNER TO anigraph;

--
-- Name: user_sessions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: anigraph
--

ALTER SEQUENCE public.user_sessions_id_seq OWNED BY public.user_sessions.id;


--
-- Name: user_taste_profiles; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.user_taste_profiles (
    id integer NOT NULL,
    user_id uuid NOT NULL,
    list_id integer,
    preferred_staff_ids integer[] DEFAULT '{}'::integer[],
    preferred_studio_ids integer[] DEFAULT '{}'::integer[],
    preferred_genre_names text[] DEFAULT '{}'::text[],
    preferred_tag_names text[] DEFAULT '{}'::text[],
    preferred_era character varying(10),
    genre_vector jsonb DEFAULT '{}'::jsonb,
    tag_vector jsonb DEFAULT '{}'::jsonb,
    staff_vector jsonb DEFAULT '{}'::jsonb,
    total_favorites integer DEFAULT 0,
    taste_summary text,
    hidden_patterns jsonb DEFAULT '[]'::jsonb,
    last_computed timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.user_taste_profiles OWNER TO anigraph;

--
-- Name: user_taste_profiles_id_seq; Type: SEQUENCE; Schema: public; Owner: anigraph
--

CREATE SEQUENCE public.user_taste_profiles_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.user_taste_profiles_id_seq OWNER TO anigraph;

--
-- Name: user_taste_profiles_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: anigraph
--

ALTER SEQUENCE public.user_taste_profiles_id_seq OWNED BY public.user_taste_profiles.id;


--
-- Name: users; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.users (
    id integer NOT NULL,
    user_id uuid DEFAULT gen_random_uuid() NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    last_active timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    email character varying(255),
    google_id character varying(255),
    name character varying(255),
    picture character varying(500),
    is_anonymous boolean DEFAULT true
);


ALTER TABLE public.users OWNER TO anigraph;

--
-- Name: TABLE users; Type: COMMENT; Schema: public; Owner: anigraph
--

COMMENT ON TABLE public.users IS 'Users can be anonymous (localStorage UUID) or authenticated (Google OAuth)';


--
-- Name: COLUMN users.is_anonymous; Type: COMMENT; Schema: public; Owner: anigraph
--

COMMENT ON COLUMN public.users.is_anonymous IS 'TRUE for localStorage users, FALSE for Google OAuth users';


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: anigraph
--

CREATE SEQUENCE public.users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.users_id_seq OWNER TO anigraph;

--
-- Name: users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: anigraph
--

ALTER SEQUENCE public.users_id_seq OWNED BY public.users.id;


--
-- Name: year_format_genre_stats; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.year_format_genre_stats (
    id integer NOT NULL,
    year integer NOT NULL,
    format character varying(20) NOT NULL,
    genre character varying(100) NOT NULL,
    total_anime integer DEFAULT 0,
    avg_score numeric(5,2)
);


ALTER TABLE public.year_format_genre_stats OWNER TO anigraph;

--
-- Name: year_format_genre_stats_id_seq; Type: SEQUENCE; Schema: public; Owner: anigraph
--

CREATE SEQUENCE public.year_format_genre_stats_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.year_format_genre_stats_id_seq OWNER TO anigraph;

--
-- Name: year_format_genre_stats_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: anigraph
--

ALTER SEQUENCE public.year_format_genre_stats_id_seq OWNED BY public.year_format_genre_stats.id;


--
-- Name: year_format_stats; Type: TABLE; Schema: public; Owner: anigraph
--

CREATE TABLE public.year_format_stats (
    id integer NOT NULL,
    year integer NOT NULL,
    format character varying(20) NOT NULL,
    total_anime integer DEFAULT 0,
    avg_score numeric(5,2)
);


ALTER TABLE public.year_format_stats OWNER TO anigraph;

--
-- Name: year_format_stats_id_seq; Type: SEQUENCE; Schema: public; Owner: anigraph
--

CREATE SEQUENCE public.year_format_stats_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.year_format_stats_id_seq OWNER TO anigraph;

--
-- Name: year_format_stats_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: anigraph
--

ALTER SEQUENCE public.year_format_stats_id_seq OWNED BY public.year_format_stats.id;


--
-- Name: anime id; Type: DEFAULT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime ALTER COLUMN id SET DEFAULT nextval('public.anime_id_seq'::regclass);


--
-- Name: audit_logs id; Type: DEFAULT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.audit_logs ALTER COLUMN id SET DEFAULT nextval('public.audit_logs_id_seq'::regclass);


--
-- Name: franchise id; Type: DEFAULT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.franchise ALTER COLUMN id SET DEFAULT nextval('public.franchise_id_seq'::regclass);


--
-- Name: genre id; Type: DEFAULT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.genre ALTER COLUMN id SET DEFAULT nextval('public.genre_id_seq'::regclass);


--
-- Name: list_computation_cache id; Type: DEFAULT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.list_computation_cache ALTER COLUMN id SET DEFAULT nextval('public.list_computation_cache_id_seq'::regclass);


--
-- Name: staff id; Type: DEFAULT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.staff ALTER COLUMN id SET DEFAULT nextval('public.staff_id_seq'::regclass);


--
-- Name: studio id; Type: DEFAULT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.studio ALTER COLUMN id SET DEFAULT nextval('public.studio_id_seq'::regclass);


--
-- Name: tag id; Type: DEFAULT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.tag ALTER COLUMN id SET DEFAULT nextval('public.tag_id_seq'::regclass);


--
-- Name: user_anime_predictions id; Type: DEFAULT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_anime_predictions ALTER COLUMN id SET DEFAULT nextval('public.user_anime_predictions_id_seq'::regclass);


--
-- Name: user_favorites id; Type: DEFAULT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_favorites ALTER COLUMN id SET DEFAULT nextval('public.user_favorites_id_seq'::regclass);


--
-- Name: user_list_items id; Type: DEFAULT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_list_items ALTER COLUMN id SET DEFAULT nextval('public.user_list_items_id_seq'::regclass);


--
-- Name: user_lists id; Type: DEFAULT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_lists ALTER COLUMN id SET DEFAULT nextval('public.user_lists_id_seq'::regclass);


--
-- Name: user_sessions id; Type: DEFAULT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_sessions ALTER COLUMN id SET DEFAULT nextval('public.user_sessions_id_seq'::regclass);


--
-- Name: user_taste_profiles id; Type: DEFAULT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_taste_profiles ALTER COLUMN id SET DEFAULT nextval('public.user_taste_profiles_id_seq'::regclass);


--
-- Name: users id; Type: DEFAULT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.users ALTER COLUMN id SET DEFAULT nextval('public.users_id_seq'::regclass);


--
-- Name: year_format_genre_stats id; Type: DEFAULT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.year_format_genre_stats ALTER COLUMN id SET DEFAULT nextval('public.year_format_genre_stats_id_seq'::regclass);


--
-- Name: year_format_stats id; Type: DEFAULT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.year_format_stats ALTER COLUMN id SET DEFAULT nextval('public.year_format_stats_id_seq'::regclass);


--
-- Name: anime anime_anilist_id_key; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime
    ADD CONSTRAINT anime_anilist_id_key UNIQUE (anilist_id);


--
-- Name: anime_genre anime_genre_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime_genre
    ADD CONSTRAINT anime_genre_pkey PRIMARY KEY (anime_id, genre_id);


--
-- Name: anime_graph_cache anime_graph_cache_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime_graph_cache
    ADD CONSTRAINT anime_graph_cache_pkey PRIMARY KEY (anilist_id);


--
-- Name: anime anime_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime
    ADD CONSTRAINT anime_pkey PRIMARY KEY (id);


--
-- Name: anime_recommendation anime_recommendation_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime_recommendation
    ADD CONSTRAINT anime_recommendation_pkey PRIMARY KEY (anime_id, recommended_anime_id);


--
-- Name: anime_relation anime_relation_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime_relation
    ADD CONSTRAINT anime_relation_pkey PRIMARY KEY (anime_id, related_anime_id);


--
-- Name: anime_sakugabooru_post anime_sakugabooru_post_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime_sakugabooru_post
    ADD CONSTRAINT anime_sakugabooru_post_pkey PRIMARY KEY (anime_id, post_id);


--
-- Name: anime_staff anime_staff_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime_staff
    ADD CONSTRAINT anime_staff_pkey PRIMARY KEY (anime_id, staff_id);


--
-- Name: anime_studio anime_studio_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime_studio
    ADD CONSTRAINT anime_studio_pkey PRIMARY KEY (anime_id, studio_id);


--
-- Name: anime_tag anime_tag_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime_tag
    ADD CONSTRAINT anime_tag_pkey PRIMARY KEY (anime_id, tag_id);


--
-- Name: audit_logs audit_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.audit_logs
    ADD CONSTRAINT audit_logs_pkey PRIMARY KEY (id);


--
-- Name: franchise franchise_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.franchise
    ADD CONSTRAINT franchise_pkey PRIMARY KEY (id);


--
-- Name: franchise franchise_title_key; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.franchise
    ADD CONSTRAINT franchise_title_key UNIQUE (title);


--
-- Name: genre genre_name_key; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.genre
    ADD CONSTRAINT genre_name_key UNIQUE (name);


--
-- Name: genre genre_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.genre
    ADD CONSTRAINT genre_pkey PRIMARY KEY (id);


--
-- Name: list_computation_cache list_computation_cache_cache_key_key; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.list_computation_cache
    ADD CONSTRAINT list_computation_cache_cache_key_key UNIQUE (cache_key);


--
-- Name: list_computation_cache list_computation_cache_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.list_computation_cache
    ADD CONSTRAINT list_computation_cache_pkey PRIMARY KEY (id);


--
-- Name: sakugabooru_post sakugabooru_post_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.sakugabooru_post
    ADD CONSTRAINT sakugabooru_post_pkey PRIMARY KEY (post_id);


--
-- Name: staff staff_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.staff
    ADD CONSTRAINT staff_pkey PRIMARY KEY (id);


--
-- Name: staff_sakugabooru_post staff_sakugabooru_post_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.staff_sakugabooru_post
    ADD CONSTRAINT staff_sakugabooru_post_pkey PRIMARY KEY (staff_id, post_id);


--
-- Name: staff staff_staff_id_key; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.staff
    ADD CONSTRAINT staff_staff_id_key UNIQUE (staff_id);


--
-- Name: studio_collaboration studio_collaboration_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.studio_collaboration
    ADD CONSTRAINT studio_collaboration_pkey PRIMARY KEY (studio_id, collaborator_id);


--
-- Name: studio studio_name_key; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.studio
    ADD CONSTRAINT studio_name_key UNIQUE (name);


--
-- Name: studio studio_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.studio
    ADD CONSTRAINT studio_pkey PRIMARY KEY (id);


--
-- Name: tag tag_name_key; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.tag
    ADD CONSTRAINT tag_name_key UNIQUE (name);


--
-- Name: tag tag_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.tag
    ADD CONSTRAINT tag_pkey PRIMARY KEY (id);


--
-- Name: user_lists unique_user_list_name; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_lists
    ADD CONSTRAINT unique_user_list_name UNIQUE (user_id, name);


--
-- Name: user_anime_predictions user_anime_predictions_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_anime_predictions
    ADD CONSTRAINT user_anime_predictions_pkey PRIMARY KEY (id);


--
-- Name: user_favorites user_favorites_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_favorites
    ADD CONSTRAINT user_favorites_pkey PRIMARY KEY (id);


--
-- Name: user_favorites user_favorites_user_id_anime_id_key; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_favorites
    ADD CONSTRAINT user_favorites_user_id_anime_id_key UNIQUE (user_id, anime_id);


--
-- Name: user_list_items user_list_items_list_id_anime_id_key; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_list_items
    ADD CONSTRAINT user_list_items_list_id_anime_id_key UNIQUE (list_id, anime_id);


--
-- Name: user_list_items user_list_items_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_list_items
    ADD CONSTRAINT user_list_items_pkey PRIMARY KEY (id);


--
-- Name: user_lists user_lists_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_lists
    ADD CONSTRAINT user_lists_pkey PRIMARY KEY (id);


--
-- Name: user_lists user_lists_share_token_key; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_lists
    ADD CONSTRAINT user_lists_share_token_key UNIQUE (share_token);


--
-- Name: user_preferences user_preferences_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_preferences
    ADD CONSTRAINT user_preferences_pkey PRIMARY KEY (user_id);


--
-- Name: user_sessions user_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_sessions
    ADD CONSTRAINT user_sessions_pkey PRIMARY KEY (id);


--
-- Name: user_sessions user_sessions_session_id_key; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_sessions
    ADD CONSTRAINT user_sessions_session_id_key UNIQUE (session_id);


--
-- Name: user_taste_profiles user_taste_profiles_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_taste_profiles
    ADD CONSTRAINT user_taste_profiles_pkey PRIMARY KEY (id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_google_id_key; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_google_id_key UNIQUE (google_id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: users users_user_id_key; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_user_id_key UNIQUE (user_id);


--
-- Name: year_format_genre_stats year_format_genre_stats_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.year_format_genre_stats
    ADD CONSTRAINT year_format_genre_stats_pkey PRIMARY KEY (id);


--
-- Name: year_format_genre_stats year_format_genre_stats_year_format_genre_key; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.year_format_genre_stats
    ADD CONSTRAINT year_format_genre_stats_year_format_genre_key UNIQUE (year, format, genre);


--
-- Name: year_format_stats year_format_stats_pkey; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.year_format_stats
    ADD CONSTRAINT year_format_stats_pkey PRIMARY KEY (id);


--
-- Name: year_format_stats year_format_stats_year_format_key; Type: CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.year_format_stats
    ADD CONSTRAINT year_format_stats_year_format_key UNIQUE (year, format);


--
-- Name: idx_anime_anidb_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_anidb_id ON public.anime USING btree (anidb_id) WHERE (anidb_id IS NOT NULL);


--
-- Name: idx_anime_anilist_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_anilist_id ON public.anime USING btree (anilist_id);


--
-- Name: idx_anime_anisearch_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_anisearch_id ON public.anime USING btree (anisearch_id) WHERE (anisearch_id IS NOT NULL);


--
-- Name: idx_anime_ann_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_ann_id ON public.anime USING btree (ann_id) WHERE (ann_id IS NOT NULL);


--
-- Name: idx_anime_average_score; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_average_score ON public.anime USING btree (average_score DESC NULLS LAST);


--
-- Name: idx_anime_community_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_community_id ON public.anime USING btree (community_id);


--
-- Name: idx_anime_episodes; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_episodes ON public.anime USING btree (episodes);


--
-- Name: idx_anime_format; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_format ON public.anime USING btree (format);


--
-- Name: idx_anime_format_score; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_format_score ON public.anime USING btree (format, average_score DESC NULLS LAST) WHERE (average_score IS NOT NULL);


--
-- Name: idx_anime_format_year_score; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_format_year_score ON public.anime USING btree (format, season_year DESC NULLS LAST, average_score DESC NULLS LAST);


--
-- Name: idx_anime_franchise_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_franchise_id ON public.anime USING btree (franchise_id);


--
-- Name: idx_anime_genre_genre_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_genre_genre_id ON public.anime_genre USING btree (genre_id);


--
-- Name: idx_anime_genre_names_gin; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_genre_names_gin ON public.anime USING gin (genre_names);


--
-- Name: idx_anime_imdb_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_imdb_id ON public.anime USING btree (imdb_id) WHERE (imdb_id IS NOT NULL);


--
-- Name: idx_anime_is_adult; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_is_adult ON public.anime USING btree (is_adult);


--
-- Name: idx_anime_kitsu_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_kitsu_id ON public.anime USING btree (kitsu_id) WHERE (kitsu_id IS NOT NULL);


--
-- Name: idx_anime_mal_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_mal_id ON public.anime USING btree (mal_id) WHERE (mal_id IS NOT NULL);


--
-- Name: idx_anime_popularity; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_popularity ON public.anime USING btree (popularity DESC NULLS LAST);


--
-- Name: idx_anime_random_rank; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_random_rank ON public.anime USING btree (random_rank);


--
-- Name: idx_anime_sakugabooru_post_anime; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_sakugabooru_post_anime ON public.anime_sakugabooru_post USING btree (anime_id);


--
-- Name: idx_anime_sakugabooru_post_post; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_sakugabooru_post_post ON public.anime_sakugabooru_post USING btree (post_id);


--
-- Name: idx_anime_sakugabooru_tag; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_sakugabooru_tag ON public.anime USING btree (sakugabooru_tag) WHERE (sakugabooru_tag IS NOT NULL);


--
-- Name: idx_anime_season; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_season ON public.anime USING btree (season);


--
-- Name: idx_anime_season_year; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_season_year ON public.anime USING btree (season_year DESC NULLS LAST);


--
-- Name: idx_anime_simkl_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_simkl_id ON public.anime USING btree (simkl_id) WHERE (simkl_id IS NOT NULL);


--
-- Name: idx_anime_staff_staff_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_staff_staff_id ON public.anime_staff USING btree (staff_id);


--
-- Name: idx_anime_studio_is_main; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_studio_is_main ON public.anime_studio USING btree (is_main) WHERE (is_main = true);


--
-- Name: idx_anime_studio_names_gin; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_studio_names_gin ON public.anime USING gin (studio_names);


--
-- Name: idx_anime_studio_studio_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_studio_studio_id ON public.anime_studio USING btree (studio_id);


--
-- Name: idx_anime_tag_names_gin; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_tag_names_gin ON public.anime USING gin (tag_names);


--
-- Name: idx_anime_tag_rank; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_tag_rank ON public.anime_tag USING btree (rank DESC NULLS LAST);


--
-- Name: idx_anime_tag_tag_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_tag_tag_id ON public.anime_tag USING btree (tag_id);


--
-- Name: idx_anime_title_english_trgm; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_title_english_trgm ON public.anime USING gin (title_english public.gin_trgm_ops);


--
-- Name: idx_anime_title_romaji_trgm; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_title_romaji_trgm ON public.anime USING gin (title_romaji public.gin_trgm_ops);


--
-- Name: idx_anime_title_trgm; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_title_trgm ON public.anime USING gin (title public.gin_trgm_ops);


--
-- Name: idx_anime_trakt_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_trakt_id ON public.anime USING btree (trakt_id) WHERE (trakt_id IS NOT NULL);


--
-- Name: idx_anime_wikidata_qid; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_wikidata_qid ON public.anime USING btree (wikidata_qid) WHERE (wikidata_qid IS NOT NULL);


--
-- Name: idx_anime_wikidata_searched_at; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_wikidata_searched_at ON public.anime USING btree (wikidata_searched_at) WHERE ((wikidata_qid IS NULL) AND (wikidata_searched_at IS NOT NULL));


--
-- Name: idx_anime_year_score; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_anime_year_score ON public.anime USING btree (season_year DESC NULLS LAST, average_score DESC NULLS LAST) WHERE (season_year IS NOT NULL);


--
-- Name: idx_audit_logs_action; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_audit_logs_action ON public.audit_logs USING btree (action);


--
-- Name: idx_audit_logs_created_at; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_audit_logs_created_at ON public.audit_logs USING btree (created_at);


--
-- Name: idx_audit_logs_resource; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_audit_logs_resource ON public.audit_logs USING btree (resource_type, resource_id);


--
-- Name: idx_audit_logs_user_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_audit_logs_user_id ON public.audit_logs USING btree (user_id);


--
-- Name: idx_genre_name; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_genre_name ON public.genre USING btree (name);


--
-- Name: idx_list_cache_accessed; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_list_cache_accessed ON public.list_computation_cache USING btree (last_accessed);


--
-- Name: idx_list_cache_key; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_list_cache_key ON public.list_computation_cache USING btree (cache_key);


--
-- Name: idx_recommendation_anime_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_recommendation_anime_id ON public.anime_recommendation USING btree (anime_id, similarity DESC);


--
-- Name: idx_recommendation_rank; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_recommendation_rank ON public.anime_recommendation USING btree (anime_id, rank);


--
-- Name: idx_relation_anime_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_relation_anime_id ON public.anime_relation USING btree (anime_id, relation_type_rank);


--
-- Name: idx_staff_community_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_staff_community_id ON public.staff USING btree (community_id);


--
-- Name: idx_staff_name_en_trgm; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_staff_name_en_trgm ON public.staff USING gin (name_en public.gin_trgm_ops);


--
-- Name: idx_staff_name_ja_trgm; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_staff_name_ja_trgm ON public.staff USING gin (name_ja public.gin_trgm_ops);


--
-- Name: idx_staff_sakugabooru_post_post; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_staff_sakugabooru_post_post ON public.staff_sakugabooru_post USING btree (post_id);


--
-- Name: idx_staff_sakugabooru_post_staff; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_staff_sakugabooru_post_staff ON public.staff_sakugabooru_post USING btree (staff_id);


--
-- Name: idx_staff_sakugabooru_tag; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_staff_sakugabooru_tag ON public.staff USING btree (sakugabooru_tag) WHERE (sakugabooru_tag IS NOT NULL);


--
-- Name: idx_staff_staff_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_staff_staff_id ON public.staff USING btree (staff_id);


--
-- Name: idx_studio_mal_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_studio_mal_id ON public.studio USING btree (mal_id);


--
-- Name: idx_studio_name_trgm; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_studio_name_trgm ON public.studio USING gin (name public.gin_trgm_ops);


--
-- Name: idx_studio_wikidata_qid; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_studio_wikidata_qid ON public.studio USING btree (wikidata_qid) WHERE (wikidata_qid IS NOT NULL);


--
-- Name: idx_tag_category; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_tag_category ON public.tag USING btree (category);


--
-- Name: idx_tag_name; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_tag_name ON public.tag USING btree (name);


--
-- Name: idx_user_anime_predictions_user_anime_list; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE UNIQUE INDEX idx_user_anime_predictions_user_anime_list ON public.user_anime_predictions USING btree (user_id, anime_id, COALESCE(list_id, 0));


--
-- Name: idx_user_anime_predictions_user_list; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_user_anime_predictions_user_list ON public.user_anime_predictions USING btree (user_id, list_id);


--
-- Name: idx_user_favorites_anime_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_user_favorites_anime_id ON public.user_favorites USING btree (anime_id);


--
-- Name: idx_user_favorites_created_at; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_user_favorites_created_at ON public.user_favorites USING btree (created_at DESC);


--
-- Name: idx_user_favorites_user_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_user_favorites_user_id ON public.user_favorites USING btree (user_id);


--
-- Name: idx_user_list_items_anime_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_user_list_items_anime_id ON public.user_list_items USING btree (anime_id);


--
-- Name: idx_user_list_items_list_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_user_list_items_list_id ON public.user_list_items USING btree (list_id);


--
-- Name: idx_user_lists_is_public; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_user_lists_is_public ON public.user_lists USING btree (is_public);


--
-- Name: idx_user_lists_list_type; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_user_lists_list_type ON public.user_lists USING btree (list_type);


--
-- Name: idx_user_lists_share_token; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_user_lists_share_token ON public.user_lists USING btree (share_token);


--
-- Name: idx_user_lists_user_favorites; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE UNIQUE INDEX idx_user_lists_user_favorites ON public.user_lists USING btree (user_id) WHERE ((list_type)::text = 'favorites'::text);


--
-- Name: idx_user_lists_user_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_user_lists_user_id ON public.user_lists USING btree (user_id);


--
-- Name: idx_user_predictions_computed_at; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_user_predictions_computed_at ON public.user_anime_predictions USING btree (computed_at);


--
-- Name: idx_user_predictions_match_score; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_user_predictions_match_score ON public.user_anime_predictions USING btree (user_id, match_score DESC);


--
-- Name: idx_user_predictions_unique; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE UNIQUE INDEX idx_user_predictions_unique ON public.user_anime_predictions USING btree (user_id, anime_id, COALESCE(list_id, 0));


--
-- Name: idx_user_predictions_user_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_user_predictions_user_id ON public.user_anime_predictions USING btree (user_id);


--
-- Name: idx_user_sessions_expires_at; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_user_sessions_expires_at ON public.user_sessions USING btree (expires_at);


--
-- Name: idx_user_sessions_session_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_user_sessions_session_id ON public.user_sessions USING btree (session_id);


--
-- Name: idx_user_sessions_user_id; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_user_sessions_user_id ON public.user_sessions USING btree (user_id);


--
-- Name: idx_user_taste_profiles_unique; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE UNIQUE INDEX idx_user_taste_profiles_unique ON public.user_taste_profiles USING btree (user_id, COALESCE(list_id, 0));


--
-- Name: idx_user_taste_profiles_user_list; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE UNIQUE INDEX idx_user_taste_profiles_user_list ON public.user_taste_profiles USING btree (user_id, COALESCE(list_id, 0));


--
-- Name: idx_year_format_genre_stats_lookup; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_year_format_genre_stats_lookup ON public.year_format_genre_stats USING btree (year, format, genre);


--
-- Name: idx_year_format_stats_lookup; Type: INDEX; Schema: public; Owner: anigraph
--

CREATE INDEX idx_year_format_stats_lookup ON public.year_format_stats USING btree (year, format);


--
-- Name: anime_genre trigger_genre_anime_count; Type: TRIGGER; Schema: public; Owner: anigraph
--

CREATE TRIGGER trigger_genre_anime_count AFTER INSERT OR DELETE ON public.anime_genre FOR EACH ROW EXECUTE FUNCTION public.update_genre_anime_count();


--
-- Name: anime_studio trigger_studio_anime_count; Type: TRIGGER; Schema: public; Owner: anigraph
--

CREATE TRIGGER trigger_studio_anime_count AFTER INSERT OR DELETE ON public.anime_studio FOR EACH ROW EXECUTE FUNCTION public.update_studio_anime_count();


--
-- Name: anime_genre trigger_sync_genre_names; Type: TRIGGER; Schema: public; Owner: anigraph
--

CREATE TRIGGER trigger_sync_genre_names AFTER INSERT OR DELETE OR UPDATE ON public.anime_genre FOR EACH ROW EXECUTE FUNCTION public.sync_anime_genre_names();


--
-- Name: anime_studio trigger_sync_studio_names; Type: TRIGGER; Schema: public; Owner: anigraph
--

CREATE TRIGGER trigger_sync_studio_names AFTER INSERT OR DELETE OR UPDATE ON public.anime_studio FOR EACH ROW EXECUTE FUNCTION public.sync_anime_studio_names();


--
-- Name: anime_tag trigger_sync_tag_names; Type: TRIGGER; Schema: public; Owner: anigraph
--

CREATE TRIGGER trigger_sync_tag_names AFTER INSERT OR DELETE OR UPDATE ON public.anime_tag FOR EACH ROW EXECUTE FUNCTION public.sync_anime_tag_names();


--
-- Name: anime_tag trigger_tag_anime_count; Type: TRIGGER; Schema: public; Owner: anigraph
--

CREATE TRIGGER trigger_tag_anime_count AFTER INSERT OR DELETE ON public.anime_tag FOR EACH ROW EXECUTE FUNCTION public.update_tag_anime_count();


--
-- Name: user_favorites trigger_update_taste_profile_favorite_count; Type: TRIGGER; Schema: public; Owner: anigraph
--

CREATE TRIGGER trigger_update_taste_profile_favorite_count AFTER INSERT OR DELETE ON public.user_favorites FOR EACH ROW EXECUTE FUNCTION public.update_taste_profile_favorite_count();


--
-- Name: user_favorites trigger_update_user_last_active; Type: TRIGGER; Schema: public; Owner: anigraph
--

CREATE TRIGGER trigger_update_user_last_active AFTER INSERT ON public.user_favorites FOR EACH ROW EXECUTE FUNCTION public.update_user_last_active();


--
-- Name: user_lists update_user_lists_timestamp; Type: TRIGGER; Schema: public; Owner: anigraph
--

CREATE TRIGGER update_user_lists_timestamp BEFORE UPDATE ON public.user_lists FOR EACH ROW EXECUTE FUNCTION public.update_user_list_timestamp();


--
-- Name: anime anime_franchise_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime
    ADD CONSTRAINT anime_franchise_id_fkey FOREIGN KEY (franchise_id) REFERENCES public.franchise(id);


--
-- Name: anime_genre anime_genre_anime_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime_genre
    ADD CONSTRAINT anime_genre_anime_id_fkey FOREIGN KEY (anime_id) REFERENCES public.anime(id) ON DELETE CASCADE;


--
-- Name: anime_genre anime_genre_genre_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime_genre
    ADD CONSTRAINT anime_genre_genre_id_fkey FOREIGN KEY (genre_id) REFERENCES public.genre(id) ON DELETE CASCADE;


--
-- Name: anime_recommendation anime_recommendation_anime_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime_recommendation
    ADD CONSTRAINT anime_recommendation_anime_id_fkey FOREIGN KEY (anime_id) REFERENCES public.anime(id) ON DELETE CASCADE;


--
-- Name: anime_recommendation anime_recommendation_recommended_anime_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime_recommendation
    ADD CONSTRAINT anime_recommendation_recommended_anime_id_fkey FOREIGN KEY (recommended_anime_id) REFERENCES public.anime(id) ON DELETE CASCADE;


--
-- Name: anime_relation anime_relation_anime_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime_relation
    ADD CONSTRAINT anime_relation_anime_id_fkey FOREIGN KEY (anime_id) REFERENCES public.anime(id) ON DELETE CASCADE;


--
-- Name: anime_relation anime_relation_related_anime_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime_relation
    ADD CONSTRAINT anime_relation_related_anime_id_fkey FOREIGN KEY (related_anime_id) REFERENCES public.anime(id) ON DELETE CASCADE;


--
-- Name: anime_sakugabooru_post anime_sakugabooru_post_anime_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime_sakugabooru_post
    ADD CONSTRAINT anime_sakugabooru_post_anime_id_fkey FOREIGN KEY (anime_id) REFERENCES public.anime(id) ON DELETE CASCADE;


--
-- Name: anime_sakugabooru_post anime_sakugabooru_post_post_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime_sakugabooru_post
    ADD CONSTRAINT anime_sakugabooru_post_post_id_fkey FOREIGN KEY (post_id) REFERENCES public.sakugabooru_post(post_id) ON DELETE CASCADE;


--
-- Name: anime_staff anime_staff_anime_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime_staff
    ADD CONSTRAINT anime_staff_anime_id_fkey FOREIGN KEY (anime_id) REFERENCES public.anime(id) ON DELETE CASCADE;


--
-- Name: anime_staff anime_staff_staff_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime_staff
    ADD CONSTRAINT anime_staff_staff_id_fkey FOREIGN KEY (staff_id) REFERENCES public.staff(id) ON DELETE CASCADE;


--
-- Name: anime_studio anime_studio_anime_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime_studio
    ADD CONSTRAINT anime_studio_anime_id_fkey FOREIGN KEY (anime_id) REFERENCES public.anime(id) ON DELETE CASCADE;


--
-- Name: anime_studio anime_studio_studio_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime_studio
    ADD CONSTRAINT anime_studio_studio_id_fkey FOREIGN KEY (studio_id) REFERENCES public.studio(id) ON DELETE CASCADE;


--
-- Name: anime_tag anime_tag_anime_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime_tag
    ADD CONSTRAINT anime_tag_anime_id_fkey FOREIGN KEY (anime_id) REFERENCES public.anime(id) ON DELETE CASCADE;


--
-- Name: anime_tag anime_tag_tag_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.anime_tag
    ADD CONSTRAINT anime_tag_tag_id_fkey FOREIGN KEY (tag_id) REFERENCES public.tag(id) ON DELETE CASCADE;


--
-- Name: audit_logs audit_logs_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.audit_logs
    ADD CONSTRAINT audit_logs_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id) ON DELETE CASCADE;


--
-- Name: staff_sakugabooru_post staff_sakugabooru_post_post_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.staff_sakugabooru_post
    ADD CONSTRAINT staff_sakugabooru_post_post_id_fkey FOREIGN KEY (post_id) REFERENCES public.sakugabooru_post(post_id) ON DELETE CASCADE;


--
-- Name: staff_sakugabooru_post staff_sakugabooru_post_staff_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.staff_sakugabooru_post
    ADD CONSTRAINT staff_sakugabooru_post_staff_id_fkey FOREIGN KEY (staff_id) REFERENCES public.staff(id) ON DELETE CASCADE;


--
-- Name: studio_collaboration studio_collaboration_collaborator_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.studio_collaboration
    ADD CONSTRAINT studio_collaboration_collaborator_id_fkey FOREIGN KEY (collaborator_id) REFERENCES public.studio(id) ON DELETE CASCADE;


--
-- Name: studio_collaboration studio_collaboration_studio_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.studio_collaboration
    ADD CONSTRAINT studio_collaboration_studio_id_fkey FOREIGN KEY (studio_id) REFERENCES public.studio(id) ON DELETE CASCADE;


--
-- Name: user_anime_predictions user_anime_predictions_anime_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_anime_predictions
    ADD CONSTRAINT user_anime_predictions_anime_id_fkey FOREIGN KEY (anime_id) REFERENCES public.anime(id) ON DELETE CASCADE;


--
-- Name: user_anime_predictions user_anime_predictions_list_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_anime_predictions
    ADD CONSTRAINT user_anime_predictions_list_id_fkey FOREIGN KEY (list_id) REFERENCES public.user_lists(id) ON DELETE CASCADE;


--
-- Name: user_anime_predictions user_anime_predictions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_anime_predictions
    ADD CONSTRAINT user_anime_predictions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id) ON DELETE CASCADE;


--
-- Name: user_favorites user_favorites_anime_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_favorites
    ADD CONSTRAINT user_favorites_anime_id_fkey FOREIGN KEY (anime_id) REFERENCES public.anime(id) ON DELETE CASCADE;


--
-- Name: user_favorites user_favorites_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_favorites
    ADD CONSTRAINT user_favorites_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id) ON DELETE CASCADE;


--
-- Name: user_list_items user_list_items_anime_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_list_items
    ADD CONSTRAINT user_list_items_anime_id_fkey FOREIGN KEY (anime_id) REFERENCES public.anime(id) ON DELETE CASCADE;


--
-- Name: user_list_items user_list_items_list_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_list_items
    ADD CONSTRAINT user_list_items_list_id_fkey FOREIGN KEY (list_id) REFERENCES public.user_lists(id) ON DELETE CASCADE;


--
-- Name: user_preferences user_preferences_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_preferences
    ADD CONSTRAINT user_preferences_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id) ON DELETE CASCADE;


--
-- Name: user_sessions user_sessions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_sessions
    ADD CONSTRAINT user_sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id) ON DELETE CASCADE;


--
-- Name: user_taste_profiles user_taste_profiles_list_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_taste_profiles
    ADD CONSTRAINT user_taste_profiles_list_id_fkey FOREIGN KEY (list_id) REFERENCES public.user_lists(id) ON DELETE CASCADE;


--
-- Name: user_taste_profiles user_taste_profiles_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: anigraph
--

ALTER TABLE ONLY public.user_taste_profiles
    ADD CONSTRAINT user_taste_profiles_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id) ON DELETE CASCADE;


--
-- Name: SCHEMA public; Type: ACL; Schema: -; Owner: anigraph
--

REVOKE USAGE ON SCHEMA public FROM PUBLIC;
GRANT ALL ON SCHEMA public TO PUBLIC;


--
-- PostgreSQL database dump complete
--

\unrestrict FbrFbliVx4C1sfrmuGDzAjgF19SJOtBinUEfmorsBJ0kvycUPtlNtoczc2dOkuY

